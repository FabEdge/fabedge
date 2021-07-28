# What is FabEdge
FabEdge is an open source edge networking solution based on kubernetes and kubeedge. It solves the problems including network configuration management,  network fragmentation，the lack of service discovery and the lack of topology awareness,communication between cloud-edge, edge-edge, etc. 

Fabedge supports weak network environments, such as 4/5G, WiFi，LoRa, etc., and supports dynamic IP addresses of edge nodes, which is suitable for scenarios such as the Internet of Things and the Internet of Vehicles.
# Features
* **Kubernetes Native Support**: Fully compatible with Kubernetes API, without additional development work, using few mature open source components，plug and play.  

* **Edge Container Network Management**：Management of the subnets allocation and  ip address assignment for edge containers.  

* **Edge-Cloud/Edge-Edge Collaboration**: Create secure tunnels between cloud and edge nodes for the collaboration between cloud and edge.  

* **Edge Community**: use the CRD of “community”to control which edge nodes can communicate with each others.  

* **Topology-aware service**: Improve service latency by giving higher priority to local endpoints, while still able to access endpoints in remote cloud.  
# Advantages
* **Standard**: fully compatible with k8s api, support any k8s cluster and plug and plan.  

* **Secure**: all communication over secure IPSEC tunnel using certificate.  

* **Easy to use**: designed using operator pattern , minimized operation effort.  
# How it works
* The cloud is a standard Kubernete cluster, and any CNI network plug-in can be used, such as Calico. Run the Kubeedge cloud component cloudcore in the cloud and edge component edgecore on the edge node, which registers the edge node to the cloud cluster.  

* Fabedge consists of three components: Operator, Connector and Agent. 
Operator runs in the cloud, which monitors node, service and other resource changes, dynamically maintains configmap for edge nodes, launchs agent for each edge node, and the agent consumes configmap information, and dynamically maintains network configurations such as tunnels, routing, and iptables rules of the node. Connector runs in the cloud, which is responsible for cloud side network configuration management, and relays traffic between the cloud and edge nodes.  

![image](https://user-images.githubusercontent.com/88021699/127309439-277bb003-5d9c-4eaf-af4f-0cd1f28158e5.png)

* FabEdge uses two channels for cloud-edge data exchange. One is the websock channel managed by kubeedge for control signals; the other is an secure tunnel managed by FabEdge itself for application data transmission.  

* Operator monitors k8s resources such as node, service, and endpoint in the cloud, and generates a configmap for each edge node, which contains the  configuration information such as the subnet, tunnel, and load balancing rules. At the same time, the operator is responsible for generating the corresponding pod for each edge node, which is used to start the agent on the node.  

* The Connector is responsible to terminate the tunnel from edge nodes, and relay traffic between the cloud and the edge nodes.  It relays on a cloud CNI plug-in to forward traffic to nodes other than the connector. It supports callico so far.  

* The edge node uses the community CNI plug-in bridge and host-local.  

* The edge node uses the community node-local-dns feature and is responsible for the domain name resolution and caching on the local node.  

* Each edge node runs an agent and consumes its own configmap including the following functions:
    - Manage the configuration file of the CNI plug-in of this node
    - Manage the security tunnel of this node
    - Manage the load balancing rules of this node, the local backend will be used first, followed by the cloud’s  

# FabEdge vs Calico、Flannel 
Fabedge is different from standard Kubernetes network plug-ins such as Calico and Flannel. These plug-ins are mainly used in the data center to solve the internal network problems of the kubernetes cluster. Fabedge solves the edge computing qutestions: after the edge node is connected to the cloud Kubernetes cluster using Kubeedge,  how to communitcate between the PODs on different edge nodes, how to community between cloud and edge etc. Currently Fabedge can seamlessly integrate with Calico and will be extended to other plugins in the near future.  

# Contributing, Support, Discussion, and Community
If you have questions, feel free to reach out to us in the following ways:

· Please send email to fabedge@beyondcent.com  
· [社区微信交流群见官网底部](http://fabedge.io)

Please submit any FabEdge bugs, issues, and feature requests to [FabEdge GitHub Issue](https://github.com/FabEdge/fabedge/issues).

# License
FabEdge is under the Apache 2.0 license. See the [LICENSE](https://github.com/FabEdge/fabedge/blob/main/LICENSE) file for details.  

