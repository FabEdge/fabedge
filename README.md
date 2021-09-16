# What is FabEdge

[![main](https://github.com/FabEdge/fabedge/actions/workflows/main.yml/badge.svg)](https://github.com/FabEdge/fabedge/actions/workflows/main.yml)
![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)
[![Releases](https://img.shields.io/github/release/fabedge/fabedge/all.svg?style=flat-square)](https://github.com/fabedge/fabedge/releases)

<img src="https://user-images.githubusercontent.com/88021699/132610524-c5adcbd3-d49a-4de4-94de-dab46d4a2ed5.jpg" width="50%">  

FabEdge is an open source edge networking solution based on kubernetes and kubeedge. It solves the problems including complex network configuration management,   network fragmentation, the lack of service discovery ability and topology awareness in edge etc. 

Fabedge supports weak network environments, such as 4/5G, WiFi，LoRa, etc., and supports dynamic IP addresses of edge nodes, which is suitable for scenarios such as the Internet of Things and the Internet of Vehicles.
# Features
* **Kubernetes Native**: Fully compatible with Kubernetes API, without any modification, applications can communicate with each others seamlessly no mater in cloud or edge.  

* **Edge Container Network Management**：Management of the subnets allocation and  ip address assignment for edge containers.  

* **Cloud-Edge/Edge-Edge Collaboration**: Secure tunnels between cloud and edge nodes for synergy between cloud and edge.  

* **Edge Community Control**:  Use K8S CRD of “community” to control which edge nodes can communicate with each others.  

* **Topology-aware service**: Improve service latency by giving higher priority to local endpoints, while still able to access endpoints in remote cloud.  
# Advantages
* **Standard**: fully compatible with k8s api, support any k8s cluster, plug and plan.  

* **Secure**: all communication over secure IPSec tunnel using certificate.  

* **Easy to use**: designed using operator pattern , minimized ongoing operation effort.  
# How it works
* The cloud is any standard Kubernete cluster with any CNI network plug-in, such as Calico. Run cloudcore, the Kubeedge cloud side component, in the cloud and edgecore, the edge side component on the edge node, which registers the edge node to the cloud cluster.  

* Fabedge consists of three components: **Operators, Connector and Agent**

![image](https://user-images.githubusercontent.com/88021699/127309439-277bb003-5d9c-4eaf-af4f-0cd1f28158e5.png)

* FabEdge uses two channels for cloud-edge data exchange. One is the websock channel managed by kubeedge for control signals; the other is an secure tunnel managed by FabEdge itself for application data exchange.  

* Operator monitors k8s resources such as node, service, and endpoint in the cloud, and creates a configmap for each edge node, which contains the  configuration information such as the subnet, tunnel, and load balancing rules. The operator is also responsible to launch the agent pod for each edge node.  

* The Connector is responsible to terminate the tunnel from edge nodes, and relay traffic between the cloud and the edge nodes.  It relays on a cloud CNI plug-in to forward traffic to nodes other than the connectors. It supports callico so far.  

* The edge node uses the k8s community CNI plug-in bridge and host-local.  

* The edge node uses the k8s community node-local-dns feature, which is responsible for the domain name resolution and caching on the local node.  

* Each edge node runs an agent and consumes its own configmap including the following functions:
    - Manage the configuration file of the CNI plug-in of this node
    - Manage the security tunnel of this node
    - Manage the load balancing rules of this node, the local backend will be used first, followed by the cloud’s  

# FabEdge vs Calico/Flannel 
Fabedge is different from generic Kubernetes network plug-ins such as Calico/Flannel. These plug-ins are used in the data centers to solve the internal network problems of the kubernetes cluster. Fabedge solves the edge computing networing qutestions:  how to communitcate between the PODs on different edge nodes, how to community between cloud and edge etc, after the edge node is connected to the cloud cluster using Kubeedge. Currently Fabedge can seamlessly integrate with Calico and will be extended to others  in the near future.  

# Guides
Get start with [this doc](docs/install.md).

# Meeting
Regular Community Meeting

| Beijing Time | Tencent Meeting Link | Status |
| :---- | :---- |:---- |
| 2021.09.09 15:00-16:00 | Meeting ID 341 406 062 or https://meeting.tencent.com/dm/ZmnDlW6IRMj3?rs=25 | End |
| 2021.09.23 15:00-16:00 | TBD | TBD |


Resources:

[Meeting notes and agenda](https://shimo.im/docs/Wwt9TdGqgVvpDHJt)  
[Meeting recordings：bilibili channel](https://space.bilibili.com/524926244?spm_id_from=333.1007.0.0)

# Contributing, Support, Discussion, and Community
If you have questions, feel free to reach out to us in the following ways:

· Please send email to fabedge@beyondcent.com  
· WeChat Group 扫码加入微信交流群  
<img src="https://user-images.githubusercontent.com/88021699/132612921-9c5b872e-f44d-4e6c-b854-16853669028a.png" width="30%"> 

Please submit any FabEdge bugs, issues, and feature requests to [FabEdge GitHub Issue](https://github.com/FabEdge/fabedge/issues).

# License
FabEdge is under the Apache 2.0 license. See the [LICENSE](https://github.com/FabEdge/fabedge/blob/main/LICENSE) file for details.  

