# FabricEdge Agent detailed design

## Summary

The Agent is responsible for the following functions  

- Maintain CNI configuration files  

- Network tunnel maintenance (e.g. IPSec)  

- Maintain iptables information  

 

Due to the instability of the edge, the IP address may change and the IP address pool may be exhausted. To ensure the availability of the tunnel and the dynamic expansion of the IP address pool, the Agent needs to maintain the CNI configuration file and tunnel configuration file dynamically.  

In order to maintain these configuration files dynamically, the Agent needs to obtain configuration information dynamically. At present, the Agent runs on edge nodes in the form of Pod, but uses the host network to avoid dependence on THE CNI plug-in. The configuration information is mounted by a ConfigMap in the Pod's specific directory, Agent  Listen for changes to the configuration file and regenerate the configuration file as soon as there are ConfigMap changes.  

 In addition to the configuration file, if the IP address of the node where the Agent resides changes, the Agent also needs to regenerate the configuration file.  

 

## Agent configuration file format  

 

The Agent configuration file consists of two parts:  

- Network information of the node where the agent resides (see the beginning of the example)  
- Network information of nodes that need to communicate with the agent (see example peers)  

```yaml
id: C=CN, O=StrongSwan, CN=edge2
name: edge2
ip: 10.20.8.4
subnets:
  - 2.16.48.192/26
peers:
  - id: C=CN, O=StrongSwan, CN=node1
    name: node1
    ip: 10.20.8.169
    subnets:
      - 10.233.0.0/16
  - id: C=CN, O=StrongSwan, CN=edge3
    name: edge3
    ip: 10.20.8.12
    subnets:
      - 2.115.203.192/26
```

## Network tunnel maintenance  

Taking the implementation of IPSec strongSwan as an example, in order to facilitate control, agent will communicate with strongSwan through ' VICI ' protocol and configure the tunnel. The tunnel configuration content is roughly as follows:  

```
root@edge2:~# swanctl --list-conns
net-cloud: IKEv1/2, no reauthentication, rekeying every 14400s
  local:  10.20.8.4
  remote: 10.20.8.141
  local public key authentication:
    id: C=CN, O=StrongSwan, CN=edge2
    certs: C=CN, O=StrongSwan, CN=edge2
  remote public key authentication:
    id: C=CN, O=StrongSwan, CN=node1
  net-cloud-child: TUNNEL, rekeying every 3600s
    local:  2.16.48.192/26
    remote: 10.233.0.0/16
```

The IPsec configurations are as follows:

```
conn net-cloud 
          left=10.10.8.4
          leftsubnet=10.10.0.0/16,3ffe:ffff:0:01ff::/64
          leftcert=edge2Cert.pem
          right=10.10.0.1
          rightsubnet=10.10.0.0/16,4000:eeee:0:01ff::/64
          rightid="C=CN, O=StrongSwan, CN=node1"
          auto=start
```

## CNI configuration file  

 With the CNI project team [bridge](https://www.cni.dev/plugins/current/main/bridge/) and [ipam](https://www.cni.dev/plugins/current/ipam/host-local/) is used as an example:

```json
{
  "cniVersion": "0.3.1",
  "name": "fabedge",
  "type": "bridge",
  "bridge": "br-fabedge",
  "isGateway": false,
  "isDefaultGateway": true,
  "forceAddress": true,
  "ipam": {
    "type": "host-local",
    "ranges": [
      [
        {
          "subnet": "2.16.48.192/26"
        }
      ]
    ]
  }
}
```

The decision to implement a custom CNI plug-in will be made later as needed  

## Maintain the iptables

 To ensure that network data can be forwarded to Pod, iptables needs to be configured:  

- Create a 'FABEDGE' chain  

- Add the 'FABEDGE' chain to the 'FORWARD' chain  

- Add filtering rules for each edgeSubnet  

 Here is the iptables information generated for '10.10.0.0/16' :  

```
-N FABEDGE
-A FORWARD -j FABEDGE
-A FABEDGE -s 2.16.48.192/26 -j ACCEPT
-A FABEDGE -d 2.16.48.192/26 -j ACCEPT
```

