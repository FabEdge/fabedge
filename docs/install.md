# FabEdge部署

FabEdge是一个专门针对边缘计算场景的，在kubernetes，kubeedge基础上构建的网络方案，主要包含以下组件：

-  **Operator**， 运行在云端任何节点，监听节点，服务等相关资源变化，自动为Agent维护配置，并管理Agent生命周期。
- **Connector**，运行在云端选定节点，负责到边缘节点的隧道的管理。
- **Agent**，运行在每个边缘节点，负责本节点的隧道，负载均衡管理。

## 前提条件

### 云端

- [kubernetes 集群（使用calico网络插件）](https://github.com/kubernetes-sigs/kubespray)
- [Kubeedge](https://kubeedge.io/en/docs/) 

### 边缘端

- 一个或多个[Kubeedge](https://kubeedge.io/en/docs/)边缘节点

## 安装步骤

### 获取Fabedge

```shell
root@node1:~# git clone https://github.com/fabedge/fabeedge.git
```

### 为[strongswan](https://www.strongswan.org/)生成证书

1. 为**每个边缘节**点生成证书， 以edge1为例，

   ```shell
   root@node1:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              107m   v1.19.3-kubeedge-v1.1.0
     node1   Ready    connector,master,node   97d    v1.19.7
   
   root@node1:~# docker run --rm -v /ipsec.d:/ipsec.d fabedge/strongswan:latest /genCert.sh edge1  
   
   root@node1:~# scp /ipsec.d/cacerts/ca.pem          <user>:<pass>@edge1:/etc/fabedge/cacerts/
   root@node1:~# scp /ipsec.d/certs/edge1_cert.pem    <user>:<pass>@edge1:/etc/fabedge/certs/
   root@node1:~# scp /ipsec.d/private/edge1_key.pem   <user>:<pass>@edge1:/etc/fabedge/private/
   root@node1:~# scp /ipsec.d/edge1.ipsec.secrets     <user>:<pass>@edge1:/etc/fabedge/ipsec.secrets
   ```

1. 为connector服务生成证书，并拷贝到**每个**运行connector服务的节点上， 以node1为例，

   ```shell
   root@node1:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              107m   v1.19.3-kubeedge-v1.1.0         
     node1   Ready    connector,master,node   97d    v1.19.7    
   
   root@node1:~# docker run --rm -v /ipsec.d:/ipsec.d fabedge/strongswan:latest /genCert.sh connector  
   
   root@node1:~# scp /ipsec.d/cacerts/ca.pem              <user>:<pass>@node1:/etc/fabedge/cacerts/
   root@node1:~# scp /ipsec.d/certs/connector_cert.pem    <user>:<pass>@node1:/etc/fabedge/certs/
   root@node1:~# scp /ipsec.d/private/connector_key.pem   <user>:<pass>@node1:/etc/fabedge/private/
   root@node1:~# scp /ipsec.d/connector.ipsec.secrets     <user>:<pass>@node1:/etc/fabedge/ipsec.secrets
   ```


###  创建namespace

创建fabedge的资源使用的namespace，默认为fabedge

```shell
root@node1:~# kubectl create ns fabedge
```

### 部署Connector

1. 在云端选取1-3个运行connector的节点，并为**每个**节点做标记，以node1为例，

   ```shell
   root@node1:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              107m   v1.19.3-kubeedge-v1.1.0         
     node1   Ready    master,node   97d    v1.19.7     
   
   root@node1:~# kubectl label no node1 node-role.kubernetes.io/connector=
   
   root@node1:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              107m   v1.19.3-kubeedge-v1.1.0         
     node1   Ready    connector,master,node   97d    v1.19.7     
   ```

4. 修改connector的配置

   按实际环境修改edgePodCIDR, ip, sbunets属性
   
   ```shell
   root@node1:~# vi ~/fabedge/deploy/connector/cm.yaml
   ```
   
   ```yaml
   data:
     connector.yaml: |
       tunnelConfig: /etc/fabedge/tunnels.yaml
       certFile: /etc/ipsec.d/certs/connector_cert.pem    
       viciSocket: /var/run/charon.vici
       # period to sync tunnel/route/rules regularly
       syncPeriod: 5m
       # CIDR used by pods on edge nodes, modify it per your env.
       edgePodCIDR: 10.10.0.0/16
       # namespace for fabedge resources
       fabedgeNS: fabedge
       debounceDuration: 5s
     tunnels.yaml: |
       # connector identity in certificate 
       id: C=CN, O=strongSwan, CN=connector
       # connector name
       name: cloud-connector
       ip: 10.20.8.169    # ip of connector to terminate tunnels   
       subnets:
       - 10.233.0.0/16  # CIDR used by pod & service in the cloud k8s cluster
   ```
   
3. 为connector创建configmap

   ```shell
   root@node1:~# kubectl apply -f ~/fabedge/deploy/connector/cm.yaml
   ```

4. 修改connector的配置

   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: connector
     namespace: fabedge
   spec:
     replicas: 3   # number of connectors, 1-3
     selector:
       matchLabels:
         app: connector
     template:
       metadata:
         labels:
           app: connector
       spec:
         affinity:
           nodeAffinity:
             requiredDuringSchedulingIgnoredDuringExecution:
               nodeSelectorTerms:
                 - matchExpressions:
                     - key: node-role.kubernetes.io/connector
                       operator: Exists
           podAntiAffinity:
             requiredDuringSchedulingIgnoredDuringExecution:
             - labelSelector:
                 matchExpressions:
                   - key: app
                     operator: In
                     values:
                     - connector
               topologyKey: kubernetes.io/hostname
         hostNetwork: true
         serviceAccountName: connector
         containers:
           - name: strongswan
             image: fabedge/strongswan
             readinessProbe:
               exec:
                 command:
                 - /usr/sbin/swanctl
                 - --version
               initialDelaySeconds: 15
               periodSeconds: 10
             securityContext:
               capabilities:
                 add: ["NET_ADMIN", "SYS_MODULE"]
             volumeMounts:
               - name: var-run
                 mountPath: /var/run/
               - name: ipsec-d
                 mountPath: /etc/ipsec.d/
                 readOnly: true
               - name: ipsec-secrets
                 mountPath: /etc/ipsec.secrets
                 readOnly: true
           - name: connector
             securityContext:
               capabilities:
                 add: ["NET_ADMIN"]
             image: fabedge/connector
             imagePullPolicy: IfNotPresent
             volumeMounts:
               - name: var-run
                 mountPath: /var/run/
               - name: connector-config
                 mountPath: /etc/fabedge/
               - name: ipsec-d
                 mountPath: /etc/ipsec.d/
                 readOnly: true
         volumes:
           - name: var-run
             emptyDir: {}
           - name: connector-config
             configMap:
               name: connector-config
           - name: ipsec-d
             hostPath:
               path: /etc/fabedge/ipsec/
           - name: ipsec-secrets
             hostPath:
               path: /etc/fabedge/ipsec/ipsec.secrets
   ```

5. 部署connector

   ```shell
   root@node1:~# kubectl apply -f ~/fabedge/deploy/connector/deploy.yaml
   ```

6. 修改calico配置

   按实际环境修改cidr属性，disabled为true

   ```shell
   root@node1:~# vi ~/fabedge/deploy/connector/ippool.yaml
   ```

   ```yaml
   apiVersion: projectcalico.org/v3
   kind: IPPool
   metadata:
     name: fabedge
   spec:
     blockSize: 26
     # CIDR used by pods on edge nodes
     cidr: 10.10.0.0/16
     natOutgoing: false
     disabled: true
   ```


9. 创建calico pool

   ```shell
   root@node1:~# calicoctl create --filename=~/fabedge/deploy/connector/ippool.yaml
   root@node1:~# calicoctl get IPPool --output yaml   # 确认pool创建成功
   ```

### 部署Operator

1. 创建Community CRD

   ```shell
   $ kubectl apply -f ~/fabedge/deploy/crds
   ```

2. 修改配置文件

   按实际环境修改edge-network-cidr

   ```shell
   root@node1:~# vi ~/fabedge/deploy/operator/fabedge-operator.yaml
   ```

   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: fabedge-operator
     namespace: fabedge
     labels:
       app: fabedge-operator
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: fabedge-operator
     template:
       metadata:
         labels:
           app: fabedge-operator
       spec:
         containers:
           - name: operator
             image: fabedge/operator:latest
             imagePullPolicy: IfNotPresent
             args:
               - -namespace=fabedge
               - -edge-network-cidr=10.10.0.0/16     # edge pod使用的网络
               - -agent-image=fabedge/agent     
               - -strongswan-image=fabedge/strongswan  
               - -connector-config=connector-config
               - -endpoint-id-format=C=CH, O=strongSwan, CN={node}
               - -v=5
         hostNetwork: true
         serviceAccountName: fabedge-operator
   ```
   
1. 创建Operator
   
   ```shell
   root@node1:~# kubectl apply -f ~/fabedge/deploy/operator
   ```
### 配置边缘节点

1. 修改edgecore配置文件

   ```shell
   root@edge1:~# vi /etc/kubeedge/config/edgecore.yaml
   ```

   禁用edgeMesh

   ```yaml
   edgeMesh:
     enable: false
   ```

   启用CNI

   ```yaml
   edged:
       enable: true
       # 默认配置，如无必要，不要修改
       cniBinDir: /opt/cni/bin
       cniCacheDirs: /var/lib/cni/cache
       cniConfDir: /etc/cni/net.d
       # 这一行默认配置文件是没有的，得自己添加  
       networkPluginName: cni
       networkPluginMTU: 1500
   ```

   配置域名和DNS

   ```yaml
   edged:
       clusterDNS: "169.254.25.10"
       clusterDomain: "cluster.local"
   ```

   > 可以用如下方法获取相关信息

   ```shell
   root@node1:~# kubectl get cm nodelocaldns -n kube-system -o jsonpath="{.data.Corefile}"
     cluster.local:53 {
       ...
       bind 169.254.25.10
       ...
     }
   # 本例中，domain为cluster.local,  dns为169.254.25.10
   ```

3. 安装CNI插件

   ```shell
   root@edge1:~# mkdir cni
   root@edge1:~# cd cni
   root@edge1:~/cni# wget https://github.com/containernetworking/plugins/releases/download/v0.9.1/cni-plugins-linux-amd64-v0.9.1.tgz
   root@edge1:~/cni# tar xvf cni-plugins-linux-amd64-v0.9.1.tgz
   root@edge1:~/cni# cp bridge host-local loopback /opt/cni/bin
   ```

4. 重启edgecore

   ```shell
   root@edge1:~# systemctl restart edgecore
   ```

5. 确认边缘节点就绪

   ```shell
   root@node1:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              107m   v1.19.3-kubeedge-v1.1.0
     node1   Ready    connector,master,node   97d    v1.19.7
   ```

### 确认服务正常启动

```shell
root@node1:~# kubectl get po -n fabedge
NAME                               READY   STATUS    RESTARTS   AGE
connector-5947d5f66-hnfbv          2/2     Running   0          11d
fabedge-agent-edge1                2/2     Running   0          22h
fabedge-operator-dbc94c45c-r7n8g   1/1     Running   0          7d6h
```
