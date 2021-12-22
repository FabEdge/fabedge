# FabEdge V0.2

## New Feature

1. One-click deployment of K8S+KubeEdge

   FabEdge is an edge container network solution, which is used only if there is a K8S+KubeEdge cluster. However, the deployment of K8S+KubeEdge is complex, resulting in a high threshold to use FabEdge. We launched the one-click deployment function of K8S+KubeEdge for users to get started quickly.  

2. Automatic Certificate Management

   Strongswan is an open source IPSec VPN management software used by FabEdge to manage tunnels. For security, it authenticates edge nodes with certificates. But assigning certificates to each edge node is a cumbersome and error-prone process. We have realized automatic management of certificates in Operator. Certificates are automatically allocated when nodes go online, which greatly reduces the operation and maintenance workload.

3. Use the Helm for installation and deployment 

   FabEdge has multiple components that are complex to configure. We use Helm (Package Manager for Kubernetes) to manage FabEdge, which simplifies the installation and deployment process and makes it easy for users to use.  

## Other updates

1. Support IPSec NAT-T

   You can set public_addresses for the connector on the cloud to support floating IP addresses for public clouds or firewall address mapping for private clouds.  

2. Improved the Iptables rules of Connetcor

   Connector automatically configudes iptables rules to allow IPSec traffic (ESP, UDP50/4500).  

3. Enable the enable-proxy function 

   For scenarios where native Kube-Proxy is used on edge nodes, you can choose to turn off FabEdge's own proxy implementation.  

