# FabEdge部署

FabEdge是一个专门针对边缘计算场景的，在kubernetes，kubeedge基础上构建的网络方案，主要包含以下组件：

- **Operator**， 运行在云端任何节点，监听节点，服务等相关资源变化，自动为Agent维护配置，并管理Agent生命周期。
- **Connector**，运行在云端选定节点，负责到边缘节点的隧道的管理。
- **Agent**，运行在每个边缘节点，负责本节点的隧道，负载均衡等配置管理。

## 前提条件

- [kubernetes (v1.19.7+,  使用calico网络插件)](https://github.com/kubernetes-sigs/kubespray )
- [Kubeedge (v1.5.0+, 至少有一个边缘节点)](https://kubeedge.io/en/docs/)  

## 安装步骤

### 关闭**rp_filter**

在**所有云端节点**执行下面命令：

```shell
root@node1:~# for i in `ls /proc/sys/net/ipv4/conf/*/rp_filter`; do  echo 0 >$i; done

#保存配置
root@node1:~# vi /etc/sysctl.conf
..
net.ipv4.conf.default.rp_filter=0
net.ipv4.conf.all.rp_filter=0
..

#确认配置生效
root@node1:~# sysctl -a | grep rp_filter | grep -v arp
..
net.ipv4.conf.cali18867a5062d.rp_filter = 0
net.ipv4.conf.cali6202a829553.rp_filter = 0
..
```

### 启动nodelocaldns服务

确认**所有边缘节点**上[nodelocaldns](https://kubernetes.io/docs/tasks/administer-cluster/nodelocaldns/)的pod启动正常

```shell
root@node1:~# kubectl get po -n kube-system -o wide| grep nodelocaldns
nodelocaldns-4m2jx                              1/1     Running     0          4h4m    10.20.8.141    node1           
nodelocaldns-p5h9k                              1/1     Running     0          3h50m   10.20.8.139    edge1      
```

### 获取Fabedge

```shell
root@node1:~# git clone https://github.com/FabEdge/fabedge.git
```

### 为[strongswan](https://www.strongswan.org/)生成证书

1. 为**每个边缘节**点生成证书， 以edge1为例，

   ```shell
   root@node1:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              107m   v1.19.3-kubeedge-v1.1.0
     node1   Ready    master,node             97d    v1.19.7
   
   # 云端执行，生成证书
   root@node1:~# docker run --rm -v /ipsec.d:/ipsec.d fabedge/strongswan:latest /genCert.sh edge1  
   
   # 登录边缘节点，在边缘节点edge1上创建目录
   root@edge1:~# mkdir -p /etc/fabedge/ipsec 
   root@edge1:~# cd /etc/fabedge/ipsec 
   root@edge1:~# mkdir -p cacerts certs private 
   
   # 将生成的证书copy到边缘节点, 
   # 注意证书名字: edge1_cert -> edgecert.pem, edge1.ipsec.secrets -> ipsec.secrets
   # “edgecert.pem”，“ipsec.secrets” 是固定名字，不能改变
   root@node1:~# scp /ipsec.d/cacerts/ca.pem          <user>@edge1:/etc/fabedge/ipsec/cacerts/ca.pem
   root@node1:~# scp /ipsec.d/certs/edge1_cert.pem    <user>@edge1:/etc/fabedge/ipsec/certs/edgecert.pem
   root@node1:~# scp /ipsec.d/private/edge1_key.pem   <user>@edge1:/etc/fabedge/ipsec/private/edge1_key.pem
   root@node1:~# scp /ipsec.d/edge1.ipsec.secrets     <user>@edge1:/etc/fabedge/ipsec/ipsec.secrets
   ```

2. 为connector服务生成证书，并拷贝到运行connector服务的节点上， 以node1为例，

   ```shell
   root@node1:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              107m   v1.19.3-kubeedge-v1.1.0         
     node1   Ready    master,node             97d    v1.19.7    
   
   # 在node1上执行, 生成证书
   root@node1:~# docker run --rm -v /ipsec.d:/ipsec.d fabedge/strongswan:latest /genCert.sh connector  
   
   # 在node1上执行，创建目录
   root@node1:~# mkdir -p /etc/fabedge/ipsec 
   root@node1:~# cd /etc/fabedge/ipsec 
   root@node1:~# mkdir -p cacerts certs private 
   
   # 在node1上执行，copy证书
   root@node1:~# cp /ipsec.d/cacerts/ca.pem                /etc/fabedge/ipsec/cacerts/ca.pem
   root@node1:~# cp /ipsec.d/certs/connector_cert.pem     /etc/fabedge/ipsec/certs/connector_cert.pem
   root@node1:~# cp /ipsec.d/private/connector_key.pem   /etc/fabedge/ipsec/private/connector_key.pem
   root@node1:~# cp /ipsec.d/connector.ipsec.secrets    /etc/fabedge/ipsec/ipsec.secrets
   ```


###  创建命名空间

创建fabedge的资源使用的namespace，默认为**fabedge**，

```shell
root@node1:~# kubectl create ns fabedge
```

### 部署Connector

1. 在云端选取一个节点运行connector，为节点做标记，以node1为例，

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
       edgePodCIDR: 10.10.0.0/16
       # namespace for fabedge resources
       fabedgeNS: fabedge
       debounceDuration: 5s
     tunnels.yaml: |
       # connector identity in certificate 
       id: C=CN, O=StrongSwan, CN=connector
       # connector name
       name: cloud-connector
       ip: 10.20.8.169    # ip address of node, which runs connector   
       subnets:
       - 10.233.0.0/18  # service cluster-ip range
       - 10.233.64.0/18 # pod ip range in cloud
   ```
   
   > **注意：**
   >
   > **edgePodCIDR**：选择一个大的网段，每个边缘节点会从中分配一个小段，每个边缘POD会从这个小段分配一个IP地址，不能和云端pod或service网段冲突。
   >
   > **ip**：运行connector服务的节点的IP地址，从边缘节点必须到这个地址可达。
   >
   > **subnets**: 需要包含service cluster-ip range和pod ip range in cloud
   >
   > ```shell
   > # 获取 service cluster ip range
   > root@node1:~# grep -rn "service-cluster-ip-range" /etc/kubernetes/manifests
   > /etc/kubernetes/manifests/kube-apiserver.yaml:50:    - --service-cluster-ip-range=10.233.0.0/18
   > 
   > # 获取 pod ip range in cloud
   > root@node1:~# calicoctl.sh get ipPool
   > NAME           CIDR             SELECTOR
   > default-pool   10.233.64.0/18   all()
   > ```
   
5. 为connector创建configmap

   ```shell
   root@node1:~# kubectl apply -f ~/fabedge/deploy/connector/cm.yaml
   ```


5. 部署connector

   ```shell
   root@node1:~# kubectl apply -f ~/fabedge/deploy/connector/deploy.yaml
   ```

6. 修改calico配置

   cidr为前面分配的edgePodCIDR，disabled为true

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
     cidr: 10.10.0.0/16
     natOutgoing: false
     disabled: true
   ```


9. 创建calico pool

   ```shell
   # 不同环境，calico的命令可能会不同
   root@node1:~# calicoctl.sh create --filename=～/fabedge/deploy/connector/ippool.yaml
   root@node1:~# calicoctl.sh get IPPool --output yaml   # 确认pool创建成功
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

   > 可以在云端执行如下操作获取相关信息
   >
   > ```shell
   > root@node1:~# kubectl get cm nodelocaldns -n kube-system -o jsonpath="{.data.Corefile}"
   >   cluster.local:53 {
   >     ...
   >     bind 169.254.25.10
   >     ...
   >   }
   > 
   > root@node1:~# grep -rn "cluster-name" /etc/kubernetes/manifests/kube-controller-manager.yaml
   >  20:    - --cluster-name=cluster.local
   > 
   >  # 本例中，domain为cluster.local,  dns为169.254.25.10
   > ```

3. 安装CNI插件

   ```shell
   root@edge1:~# mkdir -p cni /opt/cni/bin /etc/cni/net.d /var/lib/cni/cache
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

### 部署Operator

1. 创建Community CRD

   ```shell
   $ kubectl apply -f ~/fabedge/deploy/crds
   ```

2. 修改配置文件

   修改edge-pod-cidr,  使用前面edgePodCIDR

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
             image: fabedge/operator
             imagePullPolicy: IfNotPresent
             args:
               # agent所在的namespace，要跟connector, operator在同一namespace
               - -namespace=fabedge
               # 边缘节点的Pod所在的网段，根据环境配置
               - -edge-pod-cidr=10.10.0.0/16
               - -agent-image=fabedge/agent
               - -strongswan-image=fabedge/strongswan
               # connector组件所用的configmap名称
               - -connector-config=connector-config
               # 边缘节点生成的证书的ID的格式，{node}会被替换为节点名称
               - -endpoint-id-format=C=CN, O=StrongSwan, CN={node}
               - -masq-outgoing=true
               - -v=5
   ```
   
3. 创建Operator

   ```shell
   root@node1:~# kubectl apply -f ~/fabedge/deploy/operator
   ```

### 确认服务正常启动

```shell
root@node1:~# kubectl get po -n fabedge
NAME                               READY   STATUS    RESTARTS   AGE
connector-5947d5f66-hnfbv          2/2     Running   0          11d
fabedge-agent-edge1                2/2     Running   0          22h
fabedge-operator-dbc94c45c-r7n8g   1/1     Running   0          7d6h
```
