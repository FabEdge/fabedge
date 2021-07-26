# Fabedge部署手册

Fabedge是一个专门为边缘计算场景打造的，基于kubernetes，kubeedge的网络方案。

Fabedge的优势包括：

- **标准**：符合标准的kubernetes CNI规范

- **安全**：使用加密的IPSEC隧道

- **易用**：使用operator模式，免除手动运维

Fabedge包括以下组件：
-  **Operator**， 运行在云端，监听节点，服务等相关资源变化，实时为agent维护配置，并管理agent生命周期。
- **Connector**，运行在云端，负责到边缘节点的隧道的管理。
- **Agent**，运行在边缘节点，负责本节点的隧道，负载均衡管理。

## 前提条件

### 云端

- [kubernetes 集群（使用calico网络插件）](https://github.com/kubernetes-sigs/kubespray)
- [Kubeedge](https://kubeedge.io/en/docs/) 

### 边缘端

- 一个或多个[Kubeedge](https://kubeedge.io/en/docs/)边缘节点

## 安装步骤

### 获取Fabedge

```shell
$ git clone https://github.com/fabedge/fabeedge.git
```

### 生成strongswan证书

1. 为**每个边缘节**点生成证书， 以edge1为例，

   ```shell
   $ kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              107m   v1.19.3-kubeedge-v1.1.0
     node1   Ready    connector,master,node   97d    v1.19.7
   
   $ docker run --rm -v /ipsec.d:/ipsec.d fabedge/strongswan:latest /genCert.sh edge1  
   
   $ scp /ipsec.d/cacerts/ca.pem          edge1:/etc/fabedge/cacerts/
   $ scp /ipsec.d/certs/edge1_cert.pem    edge1:/etc/fabedge/certs/
   $ scp /ipsec.d/private/edge1_key.pem   edge1:/etc/fabedge/private/
   $ scp /ipsec.d/edge1.ipsec.secrets     edge1:/etc/fabedge/ipsec.secrets
   ```

1. 为connector服务生成证书，并拷贝到**每个**运行connector服务的节点上， 以node1为例，

   ```shell
   $ kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              107m   v1.19.3-kubeedge-v1.1.0         
     node1   Ready    connector,master,node   97d    v1.19.7    
   
   $ docker run --rm -v /ipsec.d:/ipsec.d fabedge/strongswan:latest /genCert.sh connector  
   
   $ scp /ipsec.d/cacerts/ca.pem          node1:/etc/fabedge/cacerts/
   $ scp /ipsec.d/certs/connector_cert.pem    node1:/etc/fabedge/certs/
   $ scp /ipsec.d/private/connector_key.pem   node1:/etc/fabedge/private/
   $ scp /ipsec.d/connector.ipsec.secrets     node1:/etc/fabedge/ipsec.secrets
   ```


###  创建namespace

fabedge的组件使用的namespace，默认为fabedge

```shell
$ kubectl create ns fabedge
```

### 部署Connector

1. 在云端选取1-3个运行connector的节点，并为**每个**节点做标记

   ```shell
   $ kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              107m   v1.19.3-kubeedge-v1.1.0         
     node1   Ready    master,node   97d    v1.19.7     
   
   $ kubectl label no node1 node-role.kubernetes.io/connector=
   
   $ kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              107m   v1.19.3-kubeedge-v1.1.0         
     node1   Ready    connector,master,node   97d    v1.19.7     
   ```

4. 修改connector的配置

   edgePodCIDR, ip, sbunets属性必须按实际环境修改。
   
   ```shell
   $ vi ~/fabedge/deploy/connector/cm.yaml
   ```
   
   ```yaml
   data:
     connector.yaml: |
       tunnelConfig: /etc/fabedge/tunnels.yaml
       certFile: /etc/ipsec.d/certs/connector_cert.pem    
       viciSocket: /var/run/charon.vici
       # period to sync tunnel/route/rules regularly
       syncPeriod: 1m
       # CIDR used by pods on edge nodes, modify it per your env.
       edgePodCIDR: 10.10.0.0/16
       # namespace for fabedge resources
       fabedgeNS: fabedge
       debounceDuration: 1s
     tunnels.yaml: |
       # connector identity in certificate 
       id: C=CN, O=strongSwan, CN=connector
       # connector name
       name: cloud-connector
       # ip of connector to terminate tunnels
       ip: 10.20.8.169
       # CIDR used by pod in the cloud
       subnets:
       - 10.233.0.0/16
   ```
   
3. 为connector创建configmap

   ```shell
   $ kubectl apply -f ~/fabedge/deploy/connector/cm.yaml
   ```

4. 部署connector

   ```shell
   $ kubectl apply -f ~/fabedge/deploy/connector/deploy.yaml
   ```

5. 修改calico配置

   cidr属性必须按实际环境修改，disabled必须为true

   ```shell
   $ vi ~/fabedge/deploy/connector/ippool.yaml
   ```

   ```yaml
   apiVersion: projectcalico.org/v3
   kind: IPPool
   metadata:
     name: fabedge
   spec:
     blockSize: 26
     # CIDR used by pods on edge nodes, modify it per your env.
     cidr: 10.10.0.0/16
     natOutgoing: false
     disabled: true
   ```


9. 创建pool

   ```shell
    $ calicoctl create --filename=~/fabedge/deploy/connector/ippool.yaml
    $ calicoctl get IPPool --output yaml   # 确认pool创建成功
   ```

### 部署Operator

1. 创建Community CRD

   ```shell
   $ kubectl apply -f ~/fabedge/deploy/crds
   ```

2. 修改配置文件

   edge-network-cidr, agent-image, strongswan-image必须按实际环境修改

   ```shell
   $ vi ~/fabedge/deploy/operator/fabedge-operator.yaml
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
               - -edge-network-cidr=10.10.0.0/16       # edge pod使用的网络
               - -agent-image=fabedge/agent:latest     # agent使用的镜像
               - -strongswan-image=strongswan:5.9.1    # strongswan使用的镜像
               - -connector-config=connector-config
               - -endpoint-id-format=C=CH, O=strongSwan, CN={node}
               - -v=5
         hostNetwork: true
         serviceAccountName: fabedge-operator
         affinity:
           nodeAffinity:
             requiredDuringSchedulingIgnoredDuringExecution:
               nodeSelectorTerms:
                 - matchExpressions:
                     - key: node-role.kubernetes.io/edge
                       operator: DoesNotExist
   ```
   
1. 创建Operator
   
   ```shell
   $ kubectl apply -f ~/fabedge/deploy/operator
   ```
### 配置边缘节点

1. 修改edgecore配置文件

   ```shell
   vi /etc/kubeedge/config/edgecore.yaml
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

   可以用如下方法获取相关信息

   ```shell
   $ kubectl get cm nodelocaldns -n kube-system -o jsonpath="{.data.Corefile}"
     cluster.local:53 {
       ...
       bind 169.254.25.10
       ...
     }
   # 本例中，domain为cluster.local,  dns为169.254.25.10
   ```

2. 重启edgecore

   ```shell
   systemctl restart edgecore
   ```

3. 安装CNI插件

   ```shell
   $ mkdir cni
   $ cd cni
   # 选择相应版本和CPU架构,下载插件
   $ wget https://github.com/containernetworking/plugins/releases/download/v0.9.1/cni-plugins-linux-amd64-v0.9.1.tgz
   $ tar xvf cni-plugins-linux-amd64-v0.9.1.tgz
   $ cp bridge host-local loopback /opt/cni/bin
   ```

