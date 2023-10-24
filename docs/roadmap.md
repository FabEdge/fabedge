# FabEdge Roadmap

## Q3 2021

- Support KubeEdge/SuperEdge/Openyurt
- Automatic management of node certificate 
- Air-gap installation
- Support Flannel/Calico
- Support IPV4
- Support IPSec Tunnel

## Q4 2021

- Support Edge Cluster
- Support topology-aware service discovery

## v0.6.0

- Support IPV6
- Implement a flexiable way to configure fabedge-agent
- Support auto networking of edge nodes in LAN

## v0.7.0

- Change the naming strategy of fabedge-agent pods
- Add commonName validation for fabedge-agent certificates
- Implement node-specific configuration of fabedge-agent arguments
- Let fabedge-agent configure sysctl parameters needed
- Let fabedge-operator manage calico ippools for CIDRs

## v0.8.0

  * Support settings strongswan's port
  * Support strongswan hole punching
  * Release fabctl which is a CLI to help diagnosing networking problems;
  * Integerate fabedge-agent with coredns and kube-proxy

## v0.9.0

 * Implement connector high  availability

 * Improve dual stack implementation

 * Improve iptables rule configuring (ensure the order of rules)

   
