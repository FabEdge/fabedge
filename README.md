# FabEdge

[![main](https://github.com/FabEdge/fabedge/actions/workflows/main.yml/badge.svg)](https://github.com/FabEdge/fabedge/actions/workflows/main.yml)
[![Releases](https://img.shields.io/github/release/fabedge/fabedge/all.svg?style=flat-square)](https://github.com/fabedge/fabedge/releases)
[![license](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/FabEdge/fabedge/blob/main/LICENSE)

<img src="https://user-images.githubusercontent.com/88021699/132610524-c5adcbd3-d49a-4de4-94de-dab46d4a2ed5.jpg" width="40%">  

English | [中文](README_zh.md)


FabEdge is a secure container networking solution based on Kubernetes, focusing on edge computing. It enables cloud-edge, edge-edge collaboration and solves the problems including complex configuration management, network isolation, unaware of the underlying topology, etc. It supports weak network, such as 4/5G, WiFi, etc. The main use cases are IoT, IoV, smart city, etc.

FabEdge supports the major edge computing frameworks ,like KubeEdge/SuperEdge/OpenYurt.

FabEdge not only supports edge nodes (remote nodes joined to the cluster via an edge computing framework such as KubeEdge), but also edge clusters (standalone K8S clusters).

FabEdge is a sandbox project of the Cloud Native Computing Foundation (CNCF).


## Features
* **Kubernetes Native**: Compatible with Kubernetes, transparent to applications.  

* **Automatic Configuration Management**: the addresses, certificates, endpoints, tunnels, etc. are automatically managed.

* **Cloud-Edge/Edge-Edge Collaboration**: Secure tunnels between cloud-edge, edge-edge nodes for synergy.

* **Topology-aware Service Discovery**: reduces service access latency, by using the nearest available service endpoint.


## Advantages:

- **Standard**: suitable for any protocol, any application.
- **Secure**: Uses mature and stable IPSec technology, and a secure certificate-based authentication system.
- **Easy to use**: Adopts the `Operator` pattern to automatically manage addresses, nodes, certificates, etc., minimizing human intervention.


## How it works
<img src="docs/images/FabEdge-Arch.png" alt="fabedge-arch" />

* The cloud can be any Kubernetes cluster with supported CNI network plug-in, including Calico, Flannel, etc.
* FabEdge builds a layer-3 data plane with tunnels in additional to the control plan managed by KubeEdge, SuperEdge, OpenYurt，etc.
* FabEdge consists of **Operators, Connector, Agent, Cloud-Agent**.
* Operator monitors k8s resources such as nodes, services, and endpoints in the cloud, and creates a configmap for each edge node, which contains the  configuration information such as the subnet, tunnel, and load balancing rules. The operator is also responsible to manage the life cycle of agent pod for each edge node.  
* Connector is responsible to terminate the tunnels from edge nodes, and forward traffic between the cloud and the edge. It relies on the cloud CNI plug-in to forward traffic to other non-connector nodes in the cloud.
* Cloud-Agent runs on the non-connector nodes in the cluster and manages the routes to remote peers.
* Each edge node runs an agent and consumes its own configmap including the following functions:
    - Manage the configuration file of the CNI plug-in of this node
    - Manage the tunnels of this node
    - Manage the load balancing rules of this node  

* Fab-DNS runs in all the clusters, to provide the topology-aware service discovery capability by intercepting the DNS queries.    


## FabEdge vs. Calico/Flannel/etc

FabEdge is not to replace the traditional Kubernetes network plugins such as Calico/Flannel. As in the above architecture diagram, Calico/Flannel is used within the cloud for communication between cloud nodes, while FabEdge is a complement to it for the edge-cloud, edge-edge communication. 

## Documentation

* [Getting started](docs/get-started.md) 
* [User Guide](docs/user-guide.md) 
* [FAQ](./docs/FAQ.md)
* [Uninstall FabEdge](docs/uninstall.md)
* [Troubleshooting](docs/troubleshooting-guide.md)


## Meeting
Regular community meeting at  2nd and 4th Thursday of every month  

Resources:  
[Meeting notes and agenda](https://shimo.im/docs/Wwt9TdGqgVvpDHJt)    
[Meeting recordings：bilibili channel](https://space.bilibili.com/524926244?spm_id_from=333.1007.0.0)  

## Contact
Any question, feel free to reach us in the following ways:

· Email: fabedge@beyondcent.com  
. Slack: [#fabedge](https://cloud-native.slack.com/archives/C03AD0TFPFF)  
· Scan the QR code to join WeChat Group

<img src="docs/images/wechat-group-qr-code.jpg" alt="wechat-group" style="width: 20%"/>



## Contributing

If you're interested in being a contributor and want to get involved in developing the FabEdge code, please see [CONTRIBUTING](./CONTRIBUTING.md) for details on submitting patches and the contribution workflow.

Please make sure to read and observe our [Code of Conduct](https://github.com/FabEdge/fabedge/blob/main/CODE_OF_CONDUCT.md).


## License
FabEdge is under the Apache 2.0 license. See the [LICENSE](https://github.com/FabEdge/fabedge/blob/main/LICENSE) file for details. 

