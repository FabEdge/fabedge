# FabEdge V0.3

## 新特性

1. 支持云端集群使用Flannel网络插件

   [Flannel](https://github.com/flannel-io/flannel)简单，易用，有大量的用户，本版本加入了对它的支持。到目前为止，FabEdge支持的插件有：Calico， Flannel。

1. 支持SuperEdge

   [SuperEdge](https://github.com/superedge/superedge/blob/main/README_CN.md)是Kubernetes原生的边缘容器方案，它将Kubernetes强大的容器管理能力扩展到边缘计算场景中，针对边缘计算场景中常见的技术挑战提供了解决方案。FabEdge本版本加入对SuperEdge的支持。
   
1. 支持OpenYurt

   [OpenYurt](https://openyurt.io/)是托管在 Cloud Native Computing Foundation (CNCF) 下的 [沙箱项目](https://www.cncf.io/sandbox-projects/). 它是基于原生 Kubernetes 构建的，目标是扩展 Kubernetes 以无缝支持边缘计算场景。FabEdge本版本加入OpenYurt的支持。

## 其它更新

1. 自动识别云端POD网段

   Operator自动识别云端集群POD网段，不再需要用户手动输入。

1. 支持用户自定义边缘节点标签

   用户可以自定义用于标识FabEdge管理的边缘节点的标签组。
