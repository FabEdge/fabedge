# FabEdge部署

FabEdge是一个专门针对边缘计算场景的，在kubernetes，kubeedge基础上构建的网络方案，主要包含以下组件：

- **Operator**， 运行在云端任何节点，监听节点，服务等相关资源变化，自动为Agent维护配置，并管理Agent生命周期。
- **Connector**，运行在云端选定节点，负责到边缘节点的隧道的管理。
- **Agent**，运行在每个边缘节点，负责本节点的隧道，负载均衡等配置管理。

## 前提条件

- [kubernetes (v1.19.7+,  使用calico网络插件)](https://github.com/kubernetes-sigs/kubespray )
- [Kubeedge (v1.5.0+, 至少有一个边缘节点)](https://kubeedge.io/en/docs/)  

如果无法满足该前提条件，请参考[部署k8s集群](https://github.com/FabEdge/fabedge/edit/main/docs/install_kubernetes.md)

## 安装步骤

### 关闭**rp_filter**

在**所有云端节点**执行下面命令：

```shell
root@master:~# for i in /proc/sys/net/ipv4/conf/*/rp_filter; do  echo 0 >$i; done

#保存配置
root@master:~# vi /etc/sysctl.conf
..
net.ipv4.conf.default.rp_filter=0
net.ipv4.conf.all.rp_filter=0
..

#确认配置生效
root@master:~# sysctl -a | grep rp_filter | grep -v arp
..
net.ipv4.conf.cali18867a5062d.rp_filter = 0
net.ipv4.conf.cali6202a829553.rp_filter = 0
..
```

### 查看nodelocaldns服务

确认**所有边缘节点**上[nodelocaldns](https://kubernetes.io/docs/tasks/administer-cluster/nodelocaldns/)的pod启动正常

```shell
root@master:~# kubectl get po -n kube-system -o wide| grep nodelocaldns
nodelocaldns-4m2jx                              1/1     Running     0          25m    10.22.45.30    master           
nodelocaldns-p5h9k                              1/1     Running     0          35m    10.22.45.26    edge1      
```

### 获取Fabedge

```shell
root@master:~# git clone https://github.com/FabEdge/fabedge.git
```



### 为[strongswan](https://www.strongswan.org/)生成证书

1. 为**每个边缘节**点生成证书， 以edge1为例，

   ```shell
   root@master:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              47m    v1.19.3-kubeedge-v1.1.0
     master  Ready    master,node             57m    v1.19.7
   
   # 云端执行，生成证书
   root@master:~# docker run --rm -v /ipsec.d:/ipsec.d fabedge/strongswan:latest /genCert.sh edge1  
   
   # 登录边缘节点，在边缘节点edge1上创建目录
   root@edge1:~# mkdir -p /etc/fabedge/ipsec 
   root@edge1:~# cd /etc/fabedge/ipsec 
   root@edge1:/etc/fabedge/ipsec# mkdir -p cacerts certs private 
   
   # 将生成的证书copy到边缘节点, 
   # 注意证书名字: edge1_cert -> edgecert.pem, edge1.ipsec.secrets -> ipsec.secrets
   # “edgecert.pem”，“ipsec.secrets” 是固定名字，不能改变
   root@master:~# scp /ipsec.d/cacerts/ca.pem          <user>@edge1:/etc/fabedge/ipsec/cacerts/ca.pem
   root@master:~# scp /ipsec.d/certs/edge1_cert.pem    <user>@edge1:/etc/fabedge/ipsec/certs/edgecert.pem
   root@master:~# scp /ipsec.d/private/edge1_key.pem   <user>@edge1:/etc/fabedge/ipsec/private/edge1_key.pem
   root@master:~# scp /ipsec.d/edge1.ipsec.secrets     <user>@edge1:/etc/fabedge/ipsec/ipsec.secrets
   ```

2. 为connector服务生成证书，并拷贝到运行connector服务的节点上， 以master为例，

   ```shell
   root@master:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              62m    v1.19.3-kubeedge-v1.1.0     
     master  Ready    master,node             72m    v1.19.7    
   
   # 在master上执行, 生成证书
   root@master:~# docker run --rm -v /ipsec.d:/ipsec.d fabedge/strongswan:latest /genCert.sh connector  
   
   # 在master上执行，创建目录
   root@master:~# mkdir -p /etc/fabedge/ipsec 
   root@master:~# cd /etc/fabedge/ipsec 
   root@master:/etc/fabedge/ipsec# mkdir -p cacerts certs private 
   
   # 在master上执行，copy证书
   root@master:~# cp /ipsec.d/cacerts/ca.pem                /etc/fabedge/ipsec/cacerts/ca.pem
   root@master:~# cp /ipsec.d/certs/connector_cert.pem     /etc/fabedge/ipsec/certs/connector_cert.pem
   root@master:~# cp /ipsec.d/private/connector_key.pem   /etc/fabedge/ipsec/private/connector_key.pem
   root@master:~# cp /ipsec.d/connector.ipsec.secrets    /etc/fabedge/ipsec/ipsec.secrets
   ```



###  创建命名空间

创建fabedge的资源使用的namespace，默认为**fabedge**，

```shell
root@master:~# kubectl create ns fabedge
```



### 部署Connector

1. 在云端选取一个节点运行connector，为节点做标记，以master为例，

   ```shell
   root@master:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              107m   v1.19.3-kubeedge-v1.1.0     
     master  Ready    master,node             117m   v1.19.7     
   
   root@master:~# kubectl label no master node-role.kubernetes.io/connector=
   
   root@master:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              108m   v1.19.3-kubeedge-v1.1.0     
     master  Ready    connector,master,node   118m   v1.19.7     
   ```

2. 修改connector的配置

   按实际环境修改edgePodCIDR, ip, sbunets属性

   ```shell
   root@master:~# vi ~/fabedge/deploy/connector/cm.yaml
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
       ip: 10.22.45.30       # ip address of node, which runs connector   
       subnets:
       - 10.233.0.0/17       # CIDR used by pod & service in the cloud cluster
       nodeSubnets:
       - 10.22.45.30/32      # IP address of all cloud cluster
       - 10.22.45.31/32
       - 10.22.45.32/32
   ```

   > ⚠️**注意：**
   >
   > **CIDR：**无类别域间路由（Classless Inter-Domain Routing、CIDR）是一个用于给用户分配IP地址以及在互联网上有效地路由IP数据包的对IP地址进行归类的方法。
   >
   > **edgePodCIDR**：选择一个大的网段，每个边缘节点会从中分配一个小段，每个边缘pod会从这个小段分配一个IP地址，不能和云端pod或service的网段冲突。
   >
   > **ip**：运行connector服务的节点的IP地址，确保边缘节点能ping通这个ip。
   >
   > ```shell
   > root@edge1:~ # ping 10.22.45.30
   > ```
   >
   > **subnets**: 需要包含service clusterIP CIDR 和 pod clusterIP CIDR
   >
   > 比如，service clusterIP CIDR 是 10.233.0.0/18，podClusterIPCIDR = 10.233.64.0/18 那么subnets是10.233.0.0/17
   >
   > 获取**service clusterIP CIDR**和**pod clusterIP CIDR**的方法如下：
   >
   > ```shell
   > # service clusterIP CIDR
   > root@master:~# grep -rn "service-cluster-ip-range" /etc/kubernetes/manifests
   > # pod clusterIP CIDR
   > root@master:~# calicoctl.sh get ipPool
   > ```
   >
   > **nodeSubnets：**需要添加所有的云端节点的ip地址

3. 为connector创建configmap

```shell
root@master:~# kubectl apply -f ~/fabedge/deploy/connector/cm.yaml
```

4. 部署connector

```shell
root@master:~# kubectl apply -f ~/fabedge/deploy/connector/deploy.yaml
```

5. 修改calico配置

cidr为前面分配的edgePodCIDR，disabled为true

```shell
root@master:~# vi ~/fabedge/deploy/connector/ippool.yaml
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

6. 创建calico pool

```shell
# 不同环境，calico的命令可能会不同
root@master:~# calicoctl.sh create --filename=/root/fabedge/deploy/connector/ippool.yaml
root@master:~# calicoctl.sh get IPPool --output yaml   # 确认pool创建成功


# 如果提示没有calicoctl.sh文件，请执行以下指令
root@master:~# export DATASTORE_TYPE=kubernetes
root@master:~# export KUBECONFIG=/etc/kubernetes/admin.conf
root@master:~# calicoctl get ipPool
NAME           CIDR             SELECTOR   
default-pool   10.231.64.0/18   all()      
fabedge        10.10.0.0/16     all()
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
       clusterDomain: "root-cluster"
   ```

   > 可以在云端执行如下操作获取相关信息
   >
   > ```shell
   > root@master:~# kubectl get cm nodelocaldns -n kube-system -o jsonpath="{.data.Corefile}"
   > root-cluster:53 {
   > ...
   > bind 169.254.25.10
   > ...
   > }
   > 
   > root@master:~# grep -rn "cluster-name" /etc/kubernetes/manifests/kube-controller-manager.yaml
   > 
   > 20:    - --cluster-name=root-cluster
   > 
   > # 本例中，domain为root-cluster,  dns为169.254.25.10
   > ```

2. 安装CNI插件

   ```shell
   root@edge1:~# mkdir -p cni /opt/cni/bin /etc/cni/net.d /var/lib/cni/cache
   root@edge1:~# cd cni
   root@edge1:~/cni# wget https://github.com/containernetworking/plugins/releases/download/v0.9.1/cni-plugins-linux-amd64-v0.9.1.tgz
   root@edge1:~/cni# tar xvf cni-plugins-linux-amd64-v0.9.1.tgz
   root@edge1:~/cni# cp bridge host-local loopback /opt/cni/bin
   ```

3. 重启edgecore

   ```shell
   root@edge1:~# systemctl restart edgecore
   ```

4. 确认边缘节点就绪

   ```shell
   root@master:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              125m   v1.19.3-kubeedge-v1.1.0
     master  Ready    connector,master,node   135m   v1.19.7
   ```



### 部署Operator

1. 创建Community CRD

   ```shell
   root@master:~# kubectl apply -f ~/fabedge/deploy/crds
   ```

2. 修改配置文件

   按实际环境修改edge-network-cidr

   ```shell
   root@master:~# vi ~/fabedge/deploy/operator/fabedge-operator.yaml
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
               - -endpoint-id-format=C=CN, O=StrongSwan, CN={node}
               - -v=5
         hostNetwork: true
         serviceAccountName: fabedge-operator
   ```

   >⚠️**注意：**
   >
   >**edge-network-cidr**为【部署Connector】中“修改connector的配置”分配的**edgePodCIDR**

3. 创建Operator

   ```shell
   root@master:~# kubectl apply -f ~/fabedge/deploy/operator
   ```



### 确认服务正常启动

```shell
root@master:~# kubectl get po -n fabedge
NAME                               READY   STATUS    RESTARTS   AGE
connector-5947d5f66-hnfbv          2/2     Running   0          35m
fabedge-agent-edge1                2/2     Running   0          22s
fabedge-operator-dbc94c45c-r7n8g   1/1     Running   0          55s
```

