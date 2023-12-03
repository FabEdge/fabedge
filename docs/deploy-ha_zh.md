# 部署高可用FabEdge

​  FabEdge在v1.0.0版本实现了高可用特性，本文展示如何部署高可用FabEdge。

*注： 有关边缘框架，DNS配置注意事项请参考[快速安装](./get-started_zh.md)，本文不再赘述。*

## 环境信息

- Kubernetes v1.22.5
- Flannel v0.19.2
- KubeEdge 1.12.2
- Helm3

### 节点信息
```shell
NAME    STATUS   ROLES                  AGE     VERSION                    INTERNAL-IP    EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION      CONTAINER-RUNTIME
edge1   Ready    agent,edge             6d21h   v1.22.6-kubeedge-v1.12.2   10.22.53.116   <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://20.10.21
edge4   Ready    agent,edge             6d22h   v1.22.6-kubeedge-v1.12.2   10.40.30.110   <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://24.0.5
harry   Ready    control-plane,master   20d     v1.22.5                    192.168.1.5    <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://24.0.5
node1   Ready    <none>                 20d     v1.22.6                    192.168.1.6    <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://24.0.5
node2   Ready    <none>                 135m    v1.22.5                    192.168.1.7    <none>        Ubuntu 20.04.6 LTS   5.4.0-166-generic   docker://24.0.5
```

harry, node1, node2是云节点，位于一个网关后，该网关公开地址是10.40.10.180，edge1, edge4也分别位于一个网络。

## 部署

1. 添加chart仓库

```shell
helm repo add fabedge https://fabedge.github.io/helm-chart
```

2. 获取集群网络信息:

```
curl -s https://fabedge.github.io/helm-chart/scripts/get_cluster_info.sh | bash -
This may take some time. Please wait.

clusterDNS               : 
clusterDomain            : kubernetes
cluster-cidr             : 10.233.64.0/18
service-cluster-ip-range : 10.233.0.0/18
```

3. 执行安装脚本

```shell
curl https://fabedge.github.io/helm-chart/scripts/quickstart.sh | bash -s -- \
	--cluster-name harry \
	--cluster-region harry \
	--cluster-zone harry \
	--cluster-role host \
	--connectors node1,node2 \
	--edges edge1,edge4 \
	--connector-public-addresses 10.40.10.180 \
	--connector-public-port 45000 \
	--connector-as-mediator true \
	--enable-keepalived true \
	--keepalived-vip 192.168.1.200 \
	--keepalived-interface enp0s3 \
	--keepalived-router-id 51 \
	--chart fabedge/fabedge
```



参数说明：

* connector-public-addresse：边缘节点可以访问的 connector的公开地址

* onnector-public-port和connector-as-mediator都不是高可用部署的必须参数，只是因为本文的环境需要配置的 ；
* enable-keepalived： 是否使用FabEdge自带的keeplived，这里选择了开启
* keepalived-vip： connector使用的虚拟IP，这个IP必须是内网地址
* keepalived-interface： 用来分配connector内网地址的接口，请确保connector节点的上用来分配地址的网卡名称相同
* keepalived-router-id： 同keepalived的virtual_router_id，用来标识不同的vrrp实例，可以不配置。

4. 确认部署正常：

```shell
root@harry:~/fabedge# kubectl get nodes -o wide
NAME    STATUS   ROLES                  AGE     VERSION                    INTERNAL-IP    EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION      CONTAINER-RUNTIME
edge1   Ready    agent,edge             7d      v1.22.6-kubeedge-v1.12.2   10.22.53.116   <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://20.10.21
edge4   Ready    agent,edge             7d1h    v1.22.6-kubeedge-v1.12.2   10.40.30.110   <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://24.0.5
harry   Ready    control-plane,master   21d     v1.22.5                    192.168.1.5    <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://24.0.5
node1   Ready    connector              21d     v1.22.6                    192.168.1.6    <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://24.0.5
node2   Ready    connector              4h55m   v1.22.5                    192.168.1.7    <none>        Ubuntu 20.04.6 LTS   5.4.0-166-generic   docker://24.0.5

root@harry:~/fabedge# kubectl get po -o wide
NAME                                READY   STATUS    RESTARTS   AGE     IP             NODE    NOMINATED NODE   READINESS GATES
fabedge-agent-55fnj                 2/2     Running   0          7m14s   10.22.53.116   edge1   <none>           <none>
fabedge-agent-vvwdz                 2/2     Running   0          7m14s   10.40.30.110   edge4   <none>           <none>
fabedge-cloud-agent-rcwqk           1/1     Running   0          7m16s   192.168.1.5    harry   <none>           <none>
fabedge-connector-7b659c4cd-475l2   3/3     Running   0          7m14s   192.168.1.7    node2   <none>           <none>
fabedge-connector-7b659c4cd-6tj2c   3/3     Running   0          7m14s   192.168.1.6    node1   <none>           <none>
fabedge-operator-5f4c5b5ffd-6cghs   1/1     Running   0          7m16s   10.233.66.21   node2   <none>           <none>

root@harry:~/fabedge# kubectl get lease
NAME        HOLDER   AGE
connector   node2    7m43s
```

​       可以看到，现在有两个connector pods在运行，这两个pod会尝试获取connector lease，获取成功的会以connector角色运行，失败的则以cloud-agent角色运行，直到获取connector lease。从上面的内容可以看出，node2的connector pod获取了connector lease。

## 手工部署

也可以手工部署高可用FabEdge，在部署前请先阅读[手工部署](./manually-install_zh.md)一文，因为步骤相同，这里只提供values.yaml例子：

```yaml
cluster:
  name: harry
  role: host
  region: harry
  zone: harry

  cniType: "flannel"
  
  clusterCIDR:
  - 10.233.64.0/18
  connectorPublicAddresses:
  - 10.40.10.180
  connectorPublicPort: 45000
  connectorAsMediator: true
  serviceClusterIPRange:
  - 10.233.0.0/18

connector:
  replicas: 2

keepalived:
  create: true
  interface: enp0s3
  routerID: 51
  vip: 192.168.1.200

agent:
  args:
    ENABLE_PROXY: "true" 
    ENABLE_DNS: "true" 
```

