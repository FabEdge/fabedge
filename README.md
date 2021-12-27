# FabEdge

[![main](https://github.com/FabEdge/fabedge/actions/workflows/main.yml/badge.svg)](https://github.com/FabEdge/fabedge/actions/workflows/main.yml)
[![Releases](https://img.shields.io/github/release/fabedge/fabedge/all.svg?style=flat-square)](https://github.com/fabedge/fabedge/releases)
[![license](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/FabEdge/fabedge/blob/main/LICENSE)

<img src="https://user-images.githubusercontent.com/88021699/132610524-c5adcbd3-d49a-4de4-94de-dab46d4a2ed5.jpg" width="40%">  

English | [中文](README_zh.md)

FabEdge is a secure edge networking solution based on Kubernetes. It enables cloud-edge, edge-edge collaboration and solves the problems including complex  configuration management, network isolation, lack of topology-aware routing, etc. 

FabEdge is able to manage the network of not only the edge nodes, but also the edge clusters. 

FabEdge supports weak transport network, such as 4/5G, WiFi, etc. It is suitable for scenarios such as IoT (Internet of Things),  IoV (Internet of Vehicles), etc.

## Features
* **Kubernetes Native**: Compatible with Kubernetes, transparent to applications.  
* **Automatic Address Management**：Management of the subnets allocation and IP address assignment for edge containers.  
* **Cloud-Edge/Edge-Edge Collaboration**: Secure tunnels between cloud-edge, edge-edge nodes for synergy.  
* **Edge Node Community Control**:  Use CRD of “community” to control which edge nodes can communicate with each others.  
* **Topology-aware service**: Improve service latency by giving higher priority to local endpoints, while still able to access endpoints in remote cloud.  

## Advantages
* **Standard**: fully compatible with Kubernetes, support any cluster, any application, plug and play.  
* **Secure**: all communication over secure IPSec tunnels with certificate based authentication.  
* **Easy to use**: designed using operator pattern, minimized ongoing operation effort.  

## How it works
<img src="docs/images/fabedge-arch.jpeg" alt="fabedge-arch" style="zoom:48%;" />

* The cloud can be any Kubernetes cluster with supported CNI network plug-in, including Calico, Flannel, etc.
* FabEdge builds a layer 3 data plane with tunnels in additional to the control plan managed by KubeEdge, SuperEdge, OpenYurt，etc.
* Fabedge consists of **Operators, Connector, Agent, Cloud-Agent**.
* Operator monitors k8s resources such as node, service, and endpoint in the cloud, and creates a configmap for each edge node, which contains the  configuration information such as the subnet, tunnel, and load balancing rules. The operator is also responsible to manage the life cycle of agent pod for each edge node.  
* Connector is responsible to terminate the tunnels from edge nodes, and forward traffic between the cloud and the edge. It relies on the cloud CNI plug-in to forward traffic to other non-connector nodes in the cloud.
* Cloud-Agent runs on the non-connector nodes in the cluster and manages the routes to remote peers.
* Each edge node runs an agent and consumes its own configmap including the following functions:
    - Manage the configuration file of the CNI plug-in of this node
    - Manage the tunnels of this node
    - Manage the load balancing rules of this node  

## Compatibility

|             | KubeEdge 1.8.0 | SuperEdge  0.5.0 | OpenYurt 0.5.0 |
| ----------- | -------------- | ---------------- | -------------- |
| FabEdge 0.3 | ✓              | ✓                | ✓              |

> It means the versions we have verified and it does not mean the others do not work. Since FabEdge does not tightly coupled with any edge-computing frameworks  it is very likely that the other version works.

## FabEdge vs Calico/Flannel 

Fabedge is different from generic Kubernetes network plug-ins such as Calico/Flannel. As in the above architecture diagram, Calico/Flannel is used in the cloud for communication between cloud nodes. Fabedge is a complement to it for the edge-cloud, edge-edge communication. 

## Guides
See  [the docs](docs/).

## Meeting
Regular community meeting at  2nd and 4th Thursday of every month  

Resources:  
[Meeting notes and agenda](https://shimo.im/docs/Wwt9TdGqgVvpDHJt)    
[Meeting recordings：bilibili channel](https://space.bilibili.com/524926244?spm_id_from=333.1007.0.0)  

## Contact
Any question, feel free to reach us in the following ways:

· Email: fabedge@beyondcent.com  
· Scan the QR code to join WeChat Group

<img src="https://user-images.githubusercontent.com/88021699/132612921-9c5b872e-f44d-4e6c-b854-16853669028a.png" width="20%">

## License
FabEdge is under the Apache 2.0 license. See the [LICENSE](https://github.com/FabEdge/fabedge/blob/main/LICENSE) file for details. 

