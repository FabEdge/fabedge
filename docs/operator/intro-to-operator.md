# Operator介绍

## 概述

Operator有几个功能
 
  * 为边缘节点分配网段
  * 为边缘节点部署fabedge-agent
  * 为边缘节点生成隧道配置和服务代理规则配置，配置由fabedge-agent消费
  * 为fabedge-connector生成隧道配置
  * 社区管理

以上功能由Operator的几个组件分别实现
  * agent controller 
  * community controller
  * connector controller
  * proxy controller

## Agent Controller

Agent controller负责如下功能：

  * 边缘节点网段分配
  * fabedge-agent部署
  * 边缘节点隧道配置管理

### 边缘节点网段分配

Agent controller监控Kubernetes集群的边缘节点，当发现一个节点没有分配网段时，会从配置的IP池内挑出一个网段分配给该节点。
网段信息会记录在该节点的annotations里，对应键值为`fabedge.io/subnets`。

如果Agent controller发现某个边缘节点已分配的网段有问题时，也会重新分配一个网段。

### fabedge-agent部署

当Agent controller发现有新增边缘节点时，会为该边缘节点创建一个fabedge-agent pod， 该pod被指定部署在新增边缘节点上，以主机网络模式运行，

### 边缘节点隧道配置管理

Agent controller监控边缘节点的变动，当发现边缘节点出现IP变动，网段分配，社区关系改变，都会重新生成隧道配置到该节点转专有的configmap上，供对应的agent处理.

## Community Controller

为了实现边缘节点之间的通信，但又不会因为太多的隧道配置对边缘节点形成不必要的负担。Fabedge提出了一个叫做社区(Community)的概念, 当一些需要边缘节点需要相互访问时，
可以把他们加入到同一个社区，然后由community controller登记这些社区关系，最后由agent-controller生成对应的隧道配置。

## Connector Controller

Connector是负责实现云边通信的云端组件，它的隧道配置由connector controller负责生成。

## Proxy Controller

Proxy Controller的功能类似kube-proxy，它监听所有的Service和EndpointSlice，当发现某个Service有Endpoint在边缘节点时，会为该节点生成一份代理规则，
这份规则最终被agent消费，生成对应的IPVS规则，这样该节点的Pod在访问该Service时，只会访问本节点的endpoint，不会去访问云端的endpoint.
