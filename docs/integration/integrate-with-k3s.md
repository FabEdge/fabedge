# 基于K3S集成FabEdge

K3S是一个轻量级Kubernetes发行版，特别适合边缘计算和云边端架构，也可以作为标准K8S集群使用。

[FabEdge](https://github.com/FabEdge/fabedge)是一个专门针对边缘计算场景设计的，基于kubernetes的容器网络方案，它符合CNI规范，可以无缝集成任何K8S环境，解决边缘计算场景下云边协同，边边协同，服务发现等难题。

## 前置条件

- 本教程基于云端master节点（云主机），和边缘node节点（内网服务器，可以访问互联网），网络环境属于单向网络。另外，因为K3S官方做了大量适配，所以也适用于其他架构环境，比如可以使用云节点加各种ARM设备搭建属于你自己的边缘集群，如树莓派
- （**重要**）云主机有内网 "nodeip"，和公网ip "publicip"，请使用你的服务器ip代替
- 本教程基于K3S最新版本，并使用docker作为容器运行时
- 对环境基于无要求，可以使用Ubuntu或CentOS，以下操作基于root用户（K3S也支持拥有root权限的其他用户部署）
- 确保所有边缘节点能够访问云端运行connector的节点
   - 如果有防火墙或安全组，必须允许ESP(50)，UDP/500，UDP/4500

## 部署K3S集群

1. 安装master，在master节点执行

```shell
#安装docker
curl -fsSL https://get.docker.com | bash -s docker --mirror Aliyun
#安装k3s master，使用 publicip 和 docker运行时
curl -sfL http://rancher-mirror.cnrancher.com/k3s/k3s-install.sh | INSTALL_K3S_MIRROR=cn INSTALL_K3S_EXEC='--docker' sh -s - --node-external-ip publicip
#部署成功后可查看
kubectl get po -A
NAMESPACE     NAME                                      READY   STATUS      RESTARTS   AGE
kube-system   coredns-7448499f4d-9ljl8                  1/1     Running     0          113s
kube-system   metrics-server-86cbb8457f-4b8wn           1/1     Running     0          113s
kube-system   local-path-provisioner-5ff76fc89d-bwhcv   1/1     Running     0          113s
kube-system   helm-install-traefik-crd-tpvv8            0/1     Completed   0          114s
kube-system   svclb-traefik-8zfbw                       2/2     Running     0          108s
kube-system   helm-install-traefik-szn6g                0/1     Completed   1          114s
kube-system   traefik-97b44b794-fhd5g                   1/1     Running     0          108s
#查看token，节点join使用
cat /var/lib/rancher/k3s/server/node-token
tokenxxxxxx
```
2. 安装node，在内网服务器执行
```shell
#安装docker
curl -fsSL https://get.docker.com | bash -s docker --mirror Aliyun
#使用token加入集群，并去除flannel组件
curl -sfL http://rancher-mirror.cnrancher.com/k3s/k3s-install.sh | INSTALL_K3S_MIRROR=cn  INSTALL_K3S_EXEC='--docker --flannel-backend none' K3S_URL=https://publicip:6443 K3S_TOKEN=tokenxxxxxx sh -
#加入集群成功之后
kubectl get node
NAME         STATUS   ROLES                  AGE   VERSION
master       Ready    control-plane,master   11m   v1.21.5+k3s2
nodename     Ready    <none>                 25s   v1.21.5+k3s2
#配置kubectl与apiserver的认证
mkdir -p $HOME/.kube
sudo cp -f /etc/rancher/k3s/k3s.yaml $HOME/.kube/config
```

3. 这样一个云加边的集群就组建好了，但是现在还没有网络访问的能力，接下来需要部署fabedge，另外获取集群配置信息，使用的是K3S默认配置
```shell
cluster-cidr             : 10.42.0.0/16
service-cluster-ip-range : 10.43.0.0/16
```
4. 安装helm

```shell
wget https://get.helm.sh/helm-v3.6.3-linux-amd64.tar.gz
tar -xf helm-v3.6.3-linux-amd64.tar.gz
cp linux-amd64/helm /usr/bin/helm 
```

## 安装部署FabEdge

1. 为**边缘节点**添加标签
```shell
kubectl label no --overwrite=true nodename node-role.kubernetes.io/edge=
node/nodename labeled
   
kubectl get node
NAME       STATUS   ROLES                  AGE   VERSION
master     Ready    control-plane,master   26m   v1.21.5+k3s2
nodename   Ready    edge                   15m   v1.21.5+k3s2
```

2. 在**云端节点**运行connector的节点，并为它做标记
```shell
kubectl label no --overwrite=true master node-role.kubernetes.io/connector=
node/master labeled

kubectl get node
NAME       STATUS   ROLES                            AGE   VERSION
master     Ready    edge                             19m   v1.21.5+k3s2
nodename   Ready    connector,control-plane,master   30m   v1.21.5+k3s2
```

3. 准备values.yaml文件
```shell
operator:
  connectorPublicAddresses: publicip
  connectorSubnets: 10.43.0.0/16
  edgeLabels: node-role.kubernetes.io/edge
  masqOutgoing: true
  enableProxy: false
cniType: flannel
```
> 说明：
>
> **connectorPublicAddresses**：主节点的地址，确保能够被边缘节点访问
>
> **connectorSubnets**：云端集群中的service使用的网段，为K3S默认的10.43.0.0/16
>
> **edgeLabels**：使用前面为边缘节点添加的标签
>
> **cniType**:：集群中使用的cni插件类型

4. 安装fabedge 

```shell
helm install fabedge --create-namespace -n fabedge -f values.yaml http://116.62.127.76/fabedge-0.3.0.tgz
```
> 如果出现错误：“Error: cannot re-use a name that is still in use”，是因为fabedge helm chart已经安装，使用以下命令卸载后重试。
>```shell
> $ helm uninstall -n fabedge fabedge
>  release "fabedge" uninstalled
>```

## 部署后验证
1. 在**管理节点**上确认FabEdge服务正常
```shell
kubectl get po -n fabedge
NAME                                READY   STATUS      RESTARTS   AGE
cert-xhmxj                          0/1     Completed   0          3m7s
fabedge-operator-5b97448c9b-zl5zg   1/1     Running     0          3m1s
fabedge-agent-nodename              2/2     Running     0          2m58s
connector-6fffdbbc64-4wz86          2/2     Running     0          3m1s
```
2. 注意部署业务pod之前，需要配置好FabEdge，之前创建的pod需要删除重建才能联通
