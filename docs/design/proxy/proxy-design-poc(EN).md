# fab-proxy design and POC experiment



## Design principles

 fab-proxy is positioned as a lightweight Kube-Proxy that runs on edge nodes and solves the problem of edge POD accessing service.  

- Edge Pod Can directly access services in the cloud center, whether they are ClusterIP or NodePort.  
- Only edge Pod is considered to access edge ClusterIP services, not edge NodePort services.  

Select the Endpoint(s) used by a Service on edge nodes in the following sequence:

1.  If an Endpoint(s) is available on this edge node, use the local Endpoint(s).

2. If UseServiceInCommunity is not enabled, use the cloud Endpoint(s). 

3.  If UseServiceInCommunity is enabled and the Commnity of this edge node has available endpoints (s), select these endpoints (s).  

kube-proxysupports both iptables and IPVS modes, but ipvs is better than iptables and is also used here.  

IPVS supports multiple scheduling algorithms as a configuration item: 

  * rr - Robin Robin

  * wrr - Weighted Round Robin

  * lc - Least-Connection

  * wlc - Weighted Least-Connection

  * lblc - Locality-Based Least-Connection

  * lblcr - Locality-Based Least-Connection with Replication

  * dh - Destination Hashing

  * sh - Source Hashing

  * sed - Shortest Expected Delay

  * nq - Never Queue

    

## Technical implementation

1. The cloud controller to monitor the Service Endpoint/EndpointSlice, based on the principle of the above, for each edge nodes generate the corresponding configuration information, write configmap.  
2. Edge node agent monitors corresponding configuration files and synchronizes local routing and IPVS configuration.  

**Note** EndpointSlice supports [Topology aware hints](https://kubernetes.io/docs/concepts/services-networking/topology-aware-hints/)the new features, performance is also better, priority in use.  



## Experimental environment

<img src="proxy.png" alt="proxy" style="zoom:70%;" />



1.  Create a Service with the name demo-svc, CluserIP 10.233.49.170, and port 80.  
2.  Create two pods, one on edge2 and one on Edge3, as Endpoints for Demo-SVC. 
3.  edge1 and edge2 are in the same community. edge2 and edge3 in one Communtiy. 
4. On edgeX， create a kube-ipvs0 interface.
5. On edgeX，create aipsec42 interface，if_id is 42.
6. Add a route (subnets corresponding to the cloud tunnel) to the ipsec42 interface 10.233.0.0/16.  
7. Add a route(A large network segment divided into small network segments for edge nodes) to the ipsec42 interface other edge nodes 2.0.0.0/16.
8. Add if_Id_in and if_Id_out to each connection of stongSwan, 42, corresponding to if_id in Step 6. 
9.  Bind CluserIP, 10.233.49.170 to the interface kube-ipvs0.  
10. Create vs in IPVS using ClusterIP 10.233.49.170 using the configured scheduling algorithm.  
11. Create RS in IPVS, use the corresponding Endpoint, and use Masq mode.  

> **Note: If a Service on an edge node uses only cloud Endpoint(s), steps 9-11 do not need to be performed, and ** **cannot be performed**.



## Result verification  
1. From edge Pod, you can access any cloud Service that is not available locally.  
2.  On edge2/3, Pod can curl 10.233.49.170.
3. On edge1，Pod can curl 10.233.49.170.