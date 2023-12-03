# 常见问题

## FabEdge是CNI实现吗？

并不是，至少不是常规意义上的CNI，它的设计目标是解决边缘场景的网络通信，在云端，依然是Flannel, Calico这些CNI在负责，边缘侧暂时由FabEdge负责，或许有一天我们能做到让Flannel，Calico工作在边缘节点上。

## FabEdge能跟哪些CNI兼容？

目前只兼容了Flannel和Calico，目前兼容Flannel的vxlan模式以及Calico的IPIP和vxlan模式，另外跟Calico协作时，Calico的存储后端不能是etcd。

## 在边缘侧分配的网段有多大？是否可以调整？如何调整？

取决于你部署Kubernetes时的配置及使用的CNI实现：

* Flannel。Flannel自身没有为节点分配PodCIDR，而是使用Kubernetes分配的PodCIDR，FabEdge在这种场景下，也会使用节点自身的PodCIDR值。如果要调整每个节点的PodCIDR大小，需要您在部署Kubernetes时修改相应的配置。

* Calico。Calico会为节点分配PodCIDR，但因为FabEdge没有能力影响这个过程，所以选择自己管理边缘侧的PodCIDR分配，这也是您部署时需要配置edgePodCIDR参数的原因。要修改PodCIDR的大小，需要修改`edge-cidr-mask-size`值，如:

  ```shell
  curl https://fabedge.github.io/helm-chart/scripts/quickstart.sh | bash -s -- \
          --cluster-name beijing \
          --cluster-role host \
          --cluster-zone beijing \
          --cluster-region beijing \
          --connectors beijing \
          --edges edge1,edge2,edge3 \
          --connector-public-addresses 10.22.45.16 \
          --cni-type calico \
          --edge-pod-cidr 10.234.0.0/16 \ # 提供边缘侧的PodCIDR池
          --edge-cidr-mask-size 26 \ # 注意是掩码长度，这里参考了Kubernetes相应参数的配置方式 
          --chart fabedge/fabedge
  ```

如果您使用[手动安装](https://github.com/FabEdge/fabedge/blob/main/docs/manually-install_zh.md)，可以参考如下values.yaml配置：

```yaml
cluster:
  name: beijing
  role: host
  region: beijing
  zone: beijing
  cniType: "calico" 
  # 配置edgePodCIDR和掩码长度
  edgePodCIDR: "10.234.0.0/16"
  edgeCIDRMaskSize: 26
  
  connectorPublicAddresses:
  - 10.22.45.16

  clusterCIDR:
  - 10.233.64.0/18
  serviceClusterIPRange:
  - 10.233.0.0/18

agent:
  args:
    # 以下两个参数仅需要在kubeedge环境打开
    ENABLE_PROXY: "true" 
    ENABLE_DNS: "true"
```

*注：后面提供配置示例时，不会再提示脚本安装或手动安装*。另外，有些参数只有在手动安装的方式下才能配置，这时也不会提供脚本安装的例子。

## 云边之间是否默认可以通信？能否关闭？

是的，云边之间默认是可以通信的，并且不能关闭。

## 云边通信的流量都由connector节点来负责，是否存在单点问题？

从v1.0.0起，FabEdge实现了connector高可用。

## 边边之间默认是否可以通信？

默认不能通信，FabEdge利用VPN隧道来打通边缘场景中隔离的网络，创建VPN隧道会消耗一定的计算资源，但边缘节点之间并不需要都连通网络，为了减少没必要的消耗，FabEdge利用Community CRD来管理边边通信，参考[这里](https://github.com/FabEdge/fabedge/blob/main/docs/user-guide_zh.md#fabedge%E7%94%A8%E6%88%B7%E6%89%8B%E5%86%8C)了解如何使用Community。

## 边边通信是否可以跨网？

可以，但FabEdge v0.8.0之前的实现效果不太好，在FabEdge v0.8.0版本中实现了打洞功能，可以较好地解决边边之间的跨网通信。该功能默认是关闭的，开启方式如下:

```shell
curl https://fabedge.github.io/helm-chart/scripts/quickstart.sh | bash -s -- \
        --cluster-name beijing \
        --cluster-role host \
        --cluster-zone beijing \
        --cluster-region beijing \
        --connectors beijing \
        --edges edge1,edge2,edge3 \
        --connector-public-addresses 10.22.45.16 \
        --connector-as-mediator true  \ # 这个参数启用打洞功能
        --chart fabedge/fabedge
```

或：

```yaml
cluster:
  name: beijing
  role: host
  region: beijing
  zone: beijing
  cniType: "flannel" 

  # 启动打洞特性
  connectorAsMediator: true
  connectorPublicAddresses:
  - 10.22.45.16

  clusterCIDR:
  - 10.233.64.0/18
  serviceClusterIPRange:
  - 10.233.0.0/18

agent:
  args:
    # 以下两个参数仅需要在kubeedge环境打开
    ENABLE_PROXY: "true" 
    ENABLE_DNS: "true"
```

## 位于同一网络内的边缘节点之间通信也需要建立隧道吗？

默认情况下，是的。如果这些节点位于同一路由器下，那么可以尝试FabEdge的自动组网功能，它的工作方式类似于flannel的host-gw模式，通过UDP广播的方式寻找其他边缘节点，为这些节点上的容器生成路由，其性能也近乎主机网络。开启的方式如下:

```yaml
cluster:
  name: beijing
  role: host
  region: beijing
  zone: beijing
  cniType: "flannel" 

  connectorPublicAddresses:
  - 10.22.46.33
  clusterCIDR:
  - 10.233.64.0/18
  serviceClusterIPRange:
  - 10.233.0.0/18

agent:
  args:
    ENABLE_PROXY: "true" 
    ENABLE_DNS: "true"
    
    # 启动自动组网
    AUTO_NETWORKING: "true"
    # fabedge-agent用来广播的地址，下面的值也是默认地址，该地址将广播范围限制在同一路由器下，基本不需要修改。
    MULTICAST_ADDRESS: "239.40.20.81:18080"
    # 广播令牌，每个边缘节点只会跟持有相同令牌的其他边缘节点通信
    MULTICAST_TOKEN: "SdY3MTJDHKUkJsHU"
```

*注：因为安装脚本里没有相应的配置，所以这里没有提供示例。*

## 节点之间是否可以直接通信？是否支持SSH访问其他节点？

FabEdge并没有实现节点之间的通信，一方面这有点麻烦，另一方面，我们不希望各个网络间的安全措施因为FabEdge被突破。

至于SSH访问，FabEdge也没实现。

## FabEdge是怎么解决边缘端的服务访问的？

取决于您选择的边缘计算框架：

* OpenYurt/SuperEdge。这些框架会在边缘节点运行自己的kube-proxy和coredns，FabEdge只是提供了网络通信能力；
* KubeEdge。在v0.8.0之前，FabEdge并没有很好地解决这个问题，但您可以自己将coredns和kube-proxy运行在边缘节点；在v0.8.0之后， FabEdge将coredns和kube-proxy集成进了fabedge-agent，从而可以向边缘侧容器提供两者的能力。

目前fabedge-agent里集成的coredns版本为1.8.0，这是目前能找到的与metaServer兼容最高可用版本；kube-proxy则是1.22.5。如果您希望亲自部署coredns和kube-proxy，可以关闭这些功能:

```shell
curl https://fabedge.github.io/helm-chart/scripts/quickstart.sh | bash -s -- \
        --cluster-name beijing \
        --cluster-role host \
        --cluster-zone beijing \
        --cluster-region beijing \
        --connectors beijing \
        --edges edge1,edge2,edge3 \
        --connector-public-addresses 10.22.45.16 \
        --enable-proxy false \ # 关闭kube-proxy
        --enable-dns false \ # 关闭coredns
        --chart fabedge/fabedge
```

或者

```yaml
cluster:
  name: beijing
  role: host
  region: beijing
  zone: beijing
  cniType: "flannel" 

  connectorPublicAddresses:
  - 10.22.45.16

  clusterCIDR:
  - 10.233.64.0/18
  serviceClusterIPRange:
  - 10.233.0.0/18

agent:
  args:
    ENABLE_PROXY: "false" 
    ENABLE_DNS: "false"
```

## 我的集群域不是cluster.local，怎么办？

如果您的集群使用OpenYurt和SuperEdge，那么什么都不用做；如果您的集群使用了KubeEdge，那么可以在部署FabEdge时，提供您集群的集群域(cluster domain):

```yaml
cluster:
  name: beijing
  role: host
  region: beijing
  zone: beijing
  cniType: "flannel" 

  connectorPublicAddresses:
  - 10.22.45.16

  clusterCIDR:
  - 10.233.64.0/18
  serviceClusterIPRange:
  - 10.233.0.0/18

agent:
  args:
    ENABLE_PROXY: "true" 
    ENABLE_DNS: "true"
    # 在这里配置集群的domain
    DNS_CLUSTER_DOMAIN: "your.domain"
```

## 我不想使用node-role.kubernetes.io/edge来标记边缘节点

FabEdge默认使用”node-role.kubernetes.io/edge“标签来识别边缘节点，但您也可以使用您希望的标签，只需要在部署时提供相应的配置:

```yaml
cluster:
  name: beijing
  role: host
  region: beijing
  zone: beijing
  cniType: "flannel" 

  connectorPublicAddresses:
  - 10.22.45.16

  clusterCIDR:
  - 10.233.64.0/18
  serviceClusterIPRange:
  - 10.233.0.0/18
  
  # 配置边缘节点的识别标签，格式就是key=value。
  edgeLabels:
    - edge-node=
    # 这里做了一个enable配置，如果您只有部分节点需要运行fabedge，可以只给这些节点打上标签fabedge-enable=true
    - fabedge-enable=true
  # 配置connector节点的识别标签
  connectorLabels:
    - connector-node=

agent:
  args:
    ENABLE_PROXY: "false" 
    ENABLE_DNS: "false"
```

需要一提的是，标签参数最好在部署后不要修改，否则可能导致FabEdge工作不正常。

## 我的云端公网不能使用500, 4500端口

​      不用担心，从FabEdge v0.8.0开始，用户可以修改connector的公开端口号。不过需要一提的是，这不是修改了connector节点的strongswan的监听端口，它依然在监听500和4500，而是修改了边缘节点向云端建立隧道时使用的端口号。另外，也不需要为500做公网端口映射，当使用非500端口创建隧道时，实际上只有4500端口参与了，所以只需要为connector 的4500端口做映射。配置方式如下：

```shell
curl https://fabedge.github.io/helm-chart/scripts/quickstart.sh | bash -s -- \
	--cluster-name beijing  \
	--cluster-role host \
	--cluster-zone beijing  \
	--cluster-region beijing \
	--connectors beijing \
	--edges edge1,edge2 \
	--connector-public-addresses 10.40.20.181 \
	--connector-public-port 45000 \ # 配置connector公开端口
	--chart fabedge/fabedge
```

或

```yaml
cluster:
  name: beijing
  role: host
  region: beijing
  zone: beijing
  cniType: "flannel" 
  
  # 配置connector公开端口
  connectorPublicPort: 45000
  connectorPublicAddresses:
  - 10.22.45.16

  clusterCIDR:
  - 10.233.64.0/18
  serviceClusterIPRange:
  - 10.233.0.0/18

agent:
  args:
    # 以下两个参数仅需要在kubeedge环境打开
    ENABLE_PROXY: "true" 
    ENABLE_DNS: "true"
```

需要一提的是，使用这种方式，会降低通信性能，具体原因参考[NAT Traversal](https://docs.strongswan.org/docs/5.9/features/natTraversal.html)

## 单集群场景下为什么有fabdns和service-hub的组件在运行，是否可以不用?

使用脚本安装时，会默认安装这两个组件，如果只是单集群场景，可以不用这两个组件，参数配置如下:

```shell
curl https://fabedge.github.io/helm-chart/scripts/quickstart.sh | bash -s -- \
	--cluster-name beijing  \
	--cluster-role host \
	--cluster-zone beijing  \
	--cluster-region beijing \
	--connectors beijing \
	--edges edge1,edge2 \
	--connector-public-addresses 10.40.20.181 \
	--enable-fabdns false \
	--chart fabedge/fabedge
```

或

```yaml
cluster:
  name: beijing
  role: host
  region: beijing
  zone: beijing
  cniType: "flannel" 
  
  connectorPublicAddresses:
  - 10.22.45.16

  clusterCIDR:
  - 10.233.64.0/18
  serviceClusterIPRange:
  - 10.233.0.0/18

agent:
  args:
    # 以下两个参数仅需要在kubeedge环境打开
    ENABLE_PROXY: "true" 
    ENABLE_DNS: "true"
    
fabDNS:
  # 禁用fabdns和service-hub
  create: false
```

## 多集群通信场景下，各个集群的网段是否可以重叠？

不能，不光容器网络的网段不能重叠，主机网络也不能。即便是单集群场景，主机网络的地址空间也不要重叠。

## 我想修改strongswan的配置，该怎么做？

很抱歉，FabEdge现在还做不到配置strongswan，您可以自己制作镜像，在镜像里完成配置。
