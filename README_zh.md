# FabEdge

[![main](https://github.com/FabEdge/fabedge/actions/workflows/main.yml/badge.svg)](https://github.com/FabEdge/fabedge/actions/workflows/main.yml)
[![Releases](https://img.shields.io/github/release/fabedge/fabedge/all.svg?style=flat-square)](https://github.com/fabedge/fabedge/releases)
[![license](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/FabEdge/fabedge/blob/main/LICENSE)

<img src="https://user-images.githubusercontent.com/88021699/132610524-c5adcbd3-d49a-4de4-94de-dab46d4a2ed5.jpg" width="40%">  

FabEdge是一个基于kubernetes构建的，专注于边缘计算的容器网络方案，支持KubeEdge/SuperEdge/OpenYurt等主流边缘计算框架。 FabEdge旨在解决边缘计算场景下网络管理复杂，跨集群通信困难，缺少能自动感知网络拓扑的服务发现等问题，使能云边、边边业务协同。FabEdge支持4/5G，WiFi等弱网环境，适用于物联网，车联网、智慧城市等场景。

FabEdge不仅支持边缘节点（通过KubeEdge等边缘计算框架加入集群的远程节点），还支持边缘集群（独立的K8S集群）。

FabEdge是托管在CNCF下的沙箱项目。

## 特性

* **自动地址管理**：自动管理边缘节点网段，自动管理边缘容器IP地址。
* **云边、边边协同**: 建立云边，边边安全隧道，使能云边，边边之间的业务协同。  
* **灵活的隧道管理**:  使用自定义资源“社区”，可以根据业务需要灵活控制边边隧道。
* **拓扑感知路由**: 使用最近的可用服务端点，减少服务访问延时。

## 优势

* **标准**: 遵从K8S CNI规范，适用于任何协议，任何应用 。
* **安全**: 使用成熟稳定的IPSec技术，使用安全的证书认证体系。 
* **易用**: 使用Operator机制，自动管理地址，节点，证书等，最大程度减少人为干预。

## 工作原理

<img src="docs/images/FabEdge-Arch.png" alt="fabedge-arch"/>

* KubeEdge等边缘计算框架建立了控制面，把边缘节点加入云端K8S集群，使得可以在边缘节点上下发Pod等资源；FabEdge在此基础上建立了一个三层的数据转发面，使得Pod和Pod之间可以直接通讯。
* 云端可以是任何K8S集群，目前支持的CNI包括Calico， Flannel。
* FabEdge使用安全隧道技术，目前支持IPSec。
* FabEdge包括的组件：Operators, Connector，Agent和Cloud-Agent。
* Operator运行在云端任意的节点，通过监听节点，服务等K8S资源，为每个Agent维护一个ConfigMap，包括了本Agent需要的路由信息，比如子网，端点，负载均衡规则等，同时为每个Agent维护一个Secret，包括CA证书，节点证书等。Operator也负责Agent自身的管理，包括创建，更新，删除等。
* Connector运行在云端选定的节点，负责管理从边缘节点发起的隧道，在边缘节点和云端集群之间转发流量。从Connector节点到云端其它非Connector节点的流量转发仍然依靠云端CNI。
* Cloud-Agent运行在集群中所有非边缘，非Connector的节点，它负责管理本节点到远端的路由。  
* Agent运行在每个边缘节点上， 它使用自己的ConfigMap和Secret的信息，发起到云端Connector和其它边缘节点的隧道，负责本节点的路由，负载均衡，iptables规则的管理。
* Fab-DNS运行在所有FabEdge的集群里，它通过截获DNS请求，提供拓扑感知的跨集群服务发现能力。

## FabEdge和传统CNI的区别 

FabEdge和现有的CNI，比如Calico，Flannel，互为补充，解决不同的问题。就像前面架构图所示，Calico等传统的插件运行在云端K8S集群里，负责云内节点之间的流量转发，FabEdge作为它的一个补充，把网络的能力延伸到了边缘节点和边缘集群，使能了云边，边边通讯。

## 用户手册

* [快速安装](docs/get-started_zh.md)
* [使用指南](docs/user-guide.md)
* [常见问题](docs/FAQ_zh.md)
* [卸载FabEdge](docs/uninstall.md)
* [问题排查指南](docs/troubleshooting-guide.md)

## 社区例会

双周例会（每个月的第一和第四周的周四下午） 

会议资料:  
[Meeting notes and agenda](https://shimo.im/docs/Wwt9TdGqgVvpDHJt)    
[Meeting recordings：bilibili channel](https://space.bilibili.com/524926244?spm_id_from=333.1007.0.0)  

## 联系方式

· 邮箱: fabedge@beyondcent.com  
· 扫描加入微信群

<img src="docs/images/wechat-group-qr-code.jpg" alt="wechat-group" style="width: 20%"/>

## 贡献

如果您有兴趣成为一个贡献者，也有兴趣加入FabEdge的开发，请查看[CONTRIBUTING](./CONTRIBUTING.md)获取更多关于如何提交 Patch 和贡献的流程

请务必阅读并遵守我们的[行为准则](./CODE_OF_CONDUCT.md)

## 软件许可

FabEdge遵循Apache 2.0 许可。
