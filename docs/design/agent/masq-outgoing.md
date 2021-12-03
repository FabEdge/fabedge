# Configure outgoing NAT

### Big picture

Configure faberge networking to perform outbound NAT for connections from pods to outside of the cluster. Fabedge source NATs the pod IP to the node IP.

### Value

The Fabedge NAT outbound connection option can be enabled, disabled with **MasqOutgoing** parameter.

### Concepts

When a pod  initiates a network connection to an IP address to outside of cluster,  the outgoing packets will have their source IP address changed from the pod IP address to the node IP address using SNAT (Source Network Address Translation). Any return packets on the connection automatically get this change reversed before being passed back to the pod.

A common use case for enabling NAT outgoing, is to allow pods with private IP addresses to connect to public IP addresses outside the cluster/the internet (subject to network policy allowing the connection, of course). 

### How to

1. Get the outgoing interface of default route, for example

   ```bash
   root@edge2:~# ip route
   default via 10.20.8.126 dev ens3 proto dhcp src 10.20.8.4 metric 100 
   ```

   Here it is **ens3**.  if can not get it,  this feature can not be used.

2. the iptables rules are generated.

   ```bash
   root@edge2:~# iptables -t nat -S
   -A POSTROUTING -j fabedge-nat-outgoing
   -A fabedge-nat-outgoing -s 10.10.48.192/26 -d 10.10.0.0/16 -j RETURN
   -A fabedge-nat-outgoing -s 10.10.48.192/26 -d 10.233.0.0/16 -j RETURN
   -A fabedge-nat-outgoing -s 10.10.48.192/26 -o ens3 -j MASQUERADE
   ```
   
   > the source is subnet used by local pods. 
   
   

