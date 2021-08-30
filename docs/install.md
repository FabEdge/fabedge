# FabEdge部署

FabEdge是一个专门针对边缘计算场景的，在kubernetes，kubeedge基础上构建的网络方案，主要包含以下组件：

- **Operator**， 运行在云端任何节点，监听节点，服务等相关资源变化，自动为Agent维护配置，并管理Agent生命周期。
- **Connector**，运行在云端选定节点，负责到边缘节点的隧道的管理。
- **Agent**，运行在每个边缘节点，负责本节点的隧道，负载均衡等配置管理。

## 前提条件

- [kubernetes (v1.19.7+,  使用calico网络插件)](https://github.com/kubernetes-sigs/kubespray )
- [Kubeedge (v1.5.0+, 至少有一个边缘节点)](https://kubeedge.io/en/docs/)  

也可以参照[文档](https://github.com/FabEdge/fabedge/blob/main/docs/install_k8s.md)快速部署一个演示集群



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

在控制节点上执行以下指令

```shell
# 确认本节点能够访问K8S API
root@master:~# kubectl get no
  NAME    STATUS   ROLES                   AGE    VERSION
  edge1   Ready    agent,edge              107m   v1.19.3    
  master  Ready    master,node             117m   v1.19.7 

root@master:~# wget http://xxxx/fabedge-cert  # TODO
root@master:~# fabedge-cert gen ca  # 生成CA密钥
root@master:~# fabedge-cert gen edge --name=cloud-connector    # 为connector生成证书密钥
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
     edge1   Ready    agent,edge              107m   v1.19.3    
     master  Ready    master,node             117m   v1.19.7     
   
   root@master:~# kubectl label no master node-role.kubernetes.io/connector=
   
   root@master:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              108m   v1.19.3-kubeedge    
     master  Ready    connector,master,node   118m   v1.19.7     
   ```

2. 部署connector

```shell
root@master:~# kubectl apply -f ~/fabedge/deploy/connector/deploy.yaml
```

3. 修改calico配置

cidr是一个大的网段，每个边缘节点会从中分配一个小段，每个边缘pod会从这个小段分配一个IP地址，不能和云端pod或service的网段冲突

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

4. 创建calico pool

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
   >**edge-network-cidr**为上面为calico分配cidr。

3. 创建Operator

   ```shell
   root@master:~# kubectl apply -f ~/fabedge/deploy/operator
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


2. 重启edgecore

   ```shell
   root@edge1:~# systemctl restart edgecore
   ```

3. 确认边缘节点就绪

   ```shell
   root@master:~# kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              125m   v1.19.3-kubeedge-v1.1.0
     master  Ready    connector,master,node   135m   v1.19.7
   ```

### 确认服务正常启动

```shell
root@master:~# kubectl get po -n fabedge
NAME                               READY   STATUS    RESTARTS   AGE
connector-5947d5f66-hnfbv          2/2     Running   0          35m
fabedge-agent-edge1                2/2     Running   0          22s
fabedge-operator-dbc94c45c-r7n8g   1/1     Running   0          55s
```
