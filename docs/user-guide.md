# FabEdge User Guide

English | [中文](user-guide_zh.md)

[toc]

## Use community

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

Create the following community to enable the communicate between `beijing` cluster and `shanghai` cluster 

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

In public cloud, the virtual machine has only private address, which prevents from FabEdge to establish the edge-to-edge tunnels. In this case, the user can apply a public address for the virtual machine and add it to the annotation of the edge node. FabEdge will use this public address to establish the tunnel instead of the private one.

```shell
#assign public address of 60.247.88.194 to node edge1
kubectl annotate node edge1 "fabedge.io/node-public-addresses=60.247.88.194"
```

## Create GlobalService

GlobalService is used to export a local/standard k8s service (ClusterIP or Headless) for other clusters to access it. And it provides the topology-aware service discovery capability.
   
1. create a service, e.g. namespace: default, name: web
2. Label it with : `fabedge.io/global-service: true`  
3. It can be accessed by the domain name: `web.defaut.svc.global`