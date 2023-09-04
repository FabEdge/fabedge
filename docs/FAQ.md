# Frequently Asked Questions (FAQ)

## Is FabEdge another CNI implementation?

Not exactly, at least it is not another CNI implementation for general purpose. It's designed for resolving issues of network communication for edge computing. On the cloud side, it still relies on Flannel or Calico to ensure network communication. But on the edge side, it is FabEdge doing the work, maybe one day we can make Flannel or Calico to be running on the edge side.

## Which CNI implementations can FabEdge work together with?

For now, FabEdge can only work with Flannel and Calico. FabEdge can work under the vxlan mode of Flannel, as well as the vxlan or IPIP mode of Calico. Furthermore, when working with Calico, you cannot use etcd as the backend storage of Calico. 

# What's the size of PodCIDR for each edge node? Can I change it? How?

Well, it's up to your Kubernetes settings and the CNI you use: 

* Flannel. Flannel doesn't allocate PodCIDR for work node itself, instead, it uses the PodCIDR field of each node and the PodCIDR is allocated by Kubernetes. In this situation, FabEdge will also use the PodCIDR of nodes. If you want to change the size, you have to set it up during the deployment of Kubernetes.

* Calico. Calico will allocate PodCIDR for each node itself, but since FabEdge is unable to change the settings of Calico, we decide to allocate PodCIDR for edge nodes ourself, that is the reason why you need to provide the value for `edge-pod-cidr` parameter. To change the size of PodCIDR, you need to set `edge-cidr-mask-size` parameter: 

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
          --edge-pod-cidr 10.234.0.0/16 \ # the address pool for edge pod, don't overlap with calico's 
          --edge-cidr-mask-size 26 \ # it's the network mask's size
          --chart fabedge/fabedge
  ```

If you choose to [install FabEdge manually](https://github.com/FabEdge/fabedge/blob/main/docs/manually-install.md), you may take the following values.yaml as an example:

```yaml
cluster:
  name: beijing
  role: host
  region: beijing
  zone: beijing
  cniType: "calico" 
  # configure edge pod CIDR and mask size for edge pods
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
    ENABLE_PROXY: "true" 
    ENABLE_DNS: "true"
```

PS: Script or manual installation will not be prompted when configuration examples are provided later. In addition, some parameters can only be configured in the manual installation mode, and no example of script installation is provided. 

## Can cloud pods and edge pods communicate to each other by default? Can I disable this feature?

Yes, cloud pods and edge pods can communicate by default and you can't disable it.

## The traffic of cloud-edge communication is handled by the connector node. Is there a Single Point Of Failure? 

Yes, for now there is no HA solution for connector, we're still working on it. 

## Is the edge-to-edge communication enabled by default?

No, FabEdge uses VPN tunnel to make communication between different available networks. However, it will use some resources to establish VPN tunnels. As not all edge nodes need to communicated, FabEdge provides community CRD to  manage communication between edge-to-edge communication to avoid unnecessary consumption. Please check out [this](https://github.com/FabEdge/fabedge/blob/main/docs/user-guide.md#use-community) and find out how to use community. 

## Is edge-to-edge communication possible across networks?

Yes, but before FabEdge v0.8.0 it didn't work well. Since v0.8.0, we implemented hole-punching feature which can help edge nodes to establish VPN tunnels across different networks.  This feature is disabled by default, you can enable it as following:

```shell
curl https://fabedge.github.io/helm-chart/scripts/quickstart.sh | bash -s -- \
        --cluster-name beijing \
        --cluster-role host \
        --cluster-zone beijing \
        --cluster-region beijing \
        --connectors beijing \
        --edges edge1,edge2,edge3 \
        --connector-public-addresses 10.22.45.16 \
        --connector-as-mediator true  \ # enable hole-punching feature
        --chart fabedge/fabedge
```

or：

```yaml
cluster:
  name: beijing
  role: host
  region: beijing
  zone: beijing
  cniType: "flannel" 

  # enable hole-punching feature
  connectorAsMediator: true
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
```

## Do edge nodes located within the same network need to establish tunnels to communicate with each other?

By default, yes it is. But if these nodes use the same router, please try auto-networking feature, it works like the host-gw mode of Flannel. Each edge node find peers under the same router using UDP multicast and generate routes for edge pods. You can enable it as following:

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
    
    # enable auto networking
    AUTO_NETWORKING: "true"
    # fabedge-agent use this address to multicast, this value is also the default value which can limit the 
    # multicast range to the router, normally you don't need to change it.
    MULTICAST_ADDRESS: "239.40.20.81:18080"
    # multicast token, each edge node can communicate with another one which has the same token
    MULTICAST_TOKEN: "SdY3MTJDHKUkJsHU"
```

## Can different nodes communicate? Does I use SSH to visit nodes?

No, FabEdge does not implement communication between nodes, which is a bit troublesome on the one hand. On the other hand, we do not want the security measures between individual networks to be breached because of FabEdge.

FabEdge doesn't provide SSH capability.

## How does edge pods visit services?

It depends on your edge computing framekwork：

* OpenYurt/SuperEdge.  They will have their own coredns and kube-proxy pods running on edge nodes and FabEdge only provide network communication.
* KubeEdge。Before v0.8.0, FabEdge didn't do much for this, but you can deploy coredns and kube-proxy on edge nodes by yourself. Since v0.8.0, FabEdge have integrated coredns and kube-proxy into fabedge-agent.

For now, the coredns integrated to fabedge-agent is 1.8.0 and kube-proxy is 1.22.5, if you want use different coredns and kube-proxy, you can turn them off: 

```shell
curl https://fabedge.github.io/helm-chart/scripts/quickstart.sh | bash -s -- \
        --cluster-name beijing \
        --cluster-role host \
        --cluster-zone beijing \
        --cluster-region beijing \
        --connectors beijing \
        --edges edge1,edge2,edge3 \
        --connector-public-addresses 10.22.45.16 \
        --enable-proxy false \ # disable kube-proxy
        --enable-dns false \ # disable coredns
        --chart fabedge/fabedge
```

or

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

## My cluster's domain is not cluster.local, what should I do?

If your cluster uses KubeEdge, you need to provide your cluster domain to FabEdge when deploying it:

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
    # configure cluster domain
    DNS_CLUSTER_DOMAIN: "your.domain"
```

## I don't want to use node-role.kubernetes.io/edge to label edge nodes

By default FabEdge uses node-role.kubernetes.io/edge to recognize edge nodes, but you can use what you like, just provide it when deploying FabEdge: 

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
  
  # configure labels which are used to recognize edge nodes. Format key=value, value can be blank
  edgeLabels:
    - edge-node=
    # Here is an enable label, sometimes you may only want fabedge-agent to running on some edge nodes,
    # you can give fabedge-enable=true label to those nodes.
    - fabedge-enable=true
  # You can also use different labels to mark connector node
  connectorLabels:
    - connector-node=

agent:
  args:
    ENABLE_PROXY: "false" 
    ENABLE_DNS: "false"
```

Don't change those parameters after you have deployed FabEdge, otherwise FabEdge might work improperly.

## I can't use 500 and 4500 as public ports for connecotr, what should I do?

Don't worry, since FabEdge v0.8.0, you can configure connector's public port. It is worth mentioning this doesn't change the listen ports of connector's strongswan, but change the port which strongswan of edge nodes use to establish tunnels. In addition, there is no need to map the public network port for 500. When a tunnel is created using a non-500 port, only port 4500 is actually used, so only port 4500 of the connector needs to be mapped. The configuration is as follows:

```shell
curl https://fabedge.github.io/helm-chart/scripts/quickstart.sh | bash -s -- \
	--cluster-name beijing  \
	--cluster-role host \
	--cluster-zone beijing  \
	--cluster-region beijing \
	--connectors beijing \
	--edges edge1,edge2 \
	--connector-public-addresses 10.40.20.181 \
	--connector-public-port 45000 \ 
	--chart fabedge/fabedge
```

or

```yaml
cluster:
  name: beijing
  role: host
  region: beijing
  zone: beijing
  cniType: "flannel" 
  
  # configure connector's public port
  connectorPublicPort: 45000
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
```

It also worth mentioning that when using this feature, it might hurt communication performance， checkout [NAT Traversal](https://docs.strongswan.org/docs/5.9/features/natTraversal.html) for why.

## Why are there fabdns and service-hub running in single cluster scenario, and can they be deleted?

If you install FabEdge using script, it will install them. If you have only one cluster, it's better to disable them. 

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

or

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
    
fabDNS:
  # disable fabdns and service-hub
  create: false
```

## Can the network addresses of each cluster overlap in a multi-cluster scenario?

No, not only the network addresses of container network, but also the host network. Even you have only one cluster, make sure the network addresses of them won't overlap.

## I want to configure strongswan, how should I do?

Sorry, for now FabEdge doesn't provide any way to do that, you may build your strongswan image and configure it in the image.
