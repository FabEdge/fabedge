# FabEdge User Guide

English | [中文](user-guide_zh.md)

[toc]

## Networking Management

### Use community

By default, the pods on the edge node can only access the pods in cloud nodes. For the pods on the edge nodes to communicate with each other directly without going through the cloud, we can define a community.

Communities can also be used to organize multiple clusters which need to communicate with each other.  

Assume there are two clusters, `beijng` and `shanghai`.  in the `beijing` cluster, there are there edge nodes of `edge1`, `edge2`, and `edge3`

Create the following community to enable the communication between edge pods on the nodes of edge1/2/3 in cluster `beijing`

```yaml
apiVersion: fabedge.io/v1alpha1
kind: Community
metadata:
  name: all-edge-nodes
spec:
  members:
    - beijing.edge1
    - beijing.edge2
    - beijing.edge3
```

Create the following community to enable the communication between `beijing` cluster and `shanghai` cluster 

```yaml
apiVersion: fabedge.io/v1alpha1
kind: Community
metadata:
  name: connectors
spec:
  members:
    - beijing.connector
    - shanghai.connector
```

### Auto networking

To facilitate networking management, FabEdge provides a feature called Auto Networking which works under LAN, it uses direct routing to let pods running edge nodes in a LAN to communicate. You need to enable it at installation, check out [manually-install](manually-install.md) for how to install fabedge manually, here is only reference values.yaml: 

```yaml
agent:
  args:
    AUTO_NETWORKING: "true" # enable auto-networking feature
    MULTICAST_TOKEN: "1b1bb567" # make sure the token is unique, only nodes with the same token can compose a network
    MULTICAST_ADDRESS: "239.40.20.81:18080" # fabedge-agent uses this address to multicast endpoints information
```

PS: Auto networking only works for edge nodes under the same router. When some nodes are in the same LAN and the same community, they will prefer auto networking.

## Register member cluster

It is required to register the endpoint information of each member cluster into the host cluster for cross-cluster communication.

1. Create a cluster resource in the host cluster: 

   ```yaml
   apiVersion: fabedge.io/v1alpha1
   kind: Cluster
   metadata:
     name: beijing
   ```

2. Get the token

   ```shell
   # kubectl describe cluster beijing
   Name:         beijing
   Namespace:    
   Kind:         Cluster
   Spec:
     Token:   eyJhbGciOi--omitted--4PebW68A
   ```
   
3. Deploy FabEdge in the member cluster using the token. 

   ```yaml
   # kubectl get cluster beijing -o yaml
   apiVersion: fabedge.io/v1alpha1
   kind: Cluster
     name: beijing
   spec:
     endPoints:
     - id: C=CN, O=fabedge.io, CN=beijing.connector
       name: beijing.connector
       nodeSubnets:
       - 10.20.8.12
       - 10.20.8.38
       publicAddresses:
       - 10.20.8.12
       subnets:
       - 10.233.0.0/18
       - 10.233.70.0/24
       - 10.233.90.0/24
       type: Connector
     token: eyJhbGciOi--omit--4PebW68A
   ```



## Assign public address for edge node

In the public cloud, the virtual machine has only private address, which prevents from FabEdge  establishing the edge-to-edge tunnels. In this case, the user can apply a public address for the virtual machine and add it to the annotation of the edge node. FabEdge will use this public address to establish the tunnel instead of the private one.

```shell
#assign public address of 60.247.88.194 to node edge1
kubectl annotate node edge1 "fabedge.io/node-public-addresses=60.247.88.194"
```

## Create GlobalService

GlobalService is used to export a local/standard k8s service (ClusterIP or Headless) for other clusters to access it. And it provides the topology-aware service discovery capability.

1. create a service, e.g. namespace: default, name: web
2. Label it with : `fabedge.io/global-service: true`  
3. It can be accessed by the domain name: `web.defaut.svc.global`

## Configure fabedge-agent for a specific node

Normally every fabedge-agent's arguments are the same, but FabEdge allows you configure arguments for a fabedge-agent on a specific node. You only need to provide fabedge agent arguments on annotations of the node, fabedge-operator will change the fabege-agent arguments. For example:  

```shell
kubectl annotate node edge1 argument.fabedge.io/enable-proxy=false # disable fab-proxy
```

The format of agent argument in node annotations is "argument.fabedge.io/argument-name", complete fabedge-agent arguments are listed [here](https://github.com/FabEdge/fabedge/blob/main/pkg/agent/config.go#L63)

## Disable fabedge-agent on specific node

fabedge-operator by default will create a fabedge-agent pod for each edge node, but FabEdge allows  you to forbid it on specific nodes. First, you need to change edge labels, check out [manually-install](manually-install.md) for how to install FabEdge manually, here is only reference values.yaml

```yaml
cluster:
  # fabedge-operator will get edge nodes with edge labels, you can change it as you like
  edgeLabels:
  - node-role.kubernetes.io/edge=
  - agent.fabedge.io/enabled=true
```

Assume you have two edge nodes: edge1 and edge2,  and you want only edge1 to have fabedge-agent, execute the command:

```yaml
kubectl label node edge1 node-role.kubernetes.io/edge=
kubectl label node edge1 agent.fabedge.io/enabled=true
```

Then you will have only edge1 have fabedge-agent running on it.

