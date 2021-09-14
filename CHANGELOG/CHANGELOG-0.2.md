# FabEdge V0.2



## 新特性

1. 一键部署K8S+KubeEdge

   FabEdge是一个边缘容器的网络方案，使用它的前提是有K8S+KubeEdge集群。但是K8S+KubeEdge的部署比较复杂，导致使用FabEdge的门槛过高。我们推出一键部署K8S+KubeEdge的功能，方便用户快速上手。

1. 自动管理证书

   Strongswan是一个开源的IPSec VPN管理软件，FabEdge底层使用它管理隧道。为了安全性，它使用证书验证边缘节点。但为每一个边缘节点分配证书是一个麻烦且容易出错的过程。我们在Operator里实现了证书的自动管理，在节点上线的时候自动分配证书，大大降低了运维工作量。
   
1. 使用Helm安装部署

   FabEdge有多个组件，组件的配置比较复杂。我们使用Helm （package manager for kubernetes）管理FabEdge，简化了安装部署过程，方便用户使用。



## 其它更新

1. 支持IPSec NAT-T

   可以为云端connector设置外网地址，public_addresses,  支持公有云使用浮动IP或私有云使用防火墙地址映射的场景。

1. 完善了connector的iptables规则

   connector自动配置iptables规则，允许IPSec流量（ESP， UDP50/4500）。

1. 增加enable-proxy的开关

   对于在边缘节点上使用原生kube-proxy的场景，可以选择关闭FabEdge自有proxy的实现。
