# FabEdge User manual

[toc]

English | [中文](user-guide_zh.md)

## Use the community

By default, the pod on the edge node can only access the cloud pod and the node, and the pod on the edge node cannot communicate with each other, in order to avoid unnecessary waste caused by the establishment of too many tunnels on the edge node. In order to make the edge nodes that need to communicate accessible to each other, we put forward the concept of community. When several edge nodes need to communicate with each other, a community can be established, and the nodes that need to communicate can be put into the list of community members, so that these community members can access each other.  

After multi-cluster communication is implemented, communities can also be used to organize clusters that need to communicate with each other.  

Creating a community is very simple. Suppose we now have an edge cluster, which we call "beijing" when deployed, and there are three edge nodes edge1, edge2, and edge3 in the cluster.  

Create the following community:  

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

> Note: The endpoint_name of a node is "cluster_name. node_name ". 

If we now want to communicate between "beijing"cluster and "shanghai" cluster ,we can create the following cluster:  

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

> Note: The member name is "cluster_name.connector".

## Register the cluster

Multi-cluster communication requires us to register endpoint information of each cluster in the host cluster:

1. Create a cluster resource in the host cluster: 

   ```yaml
   apiVersion: fabedge.io/v1alpha1
   kind: Cluster
   metadata:
     name: beijing
   ```

2. Check the token

   ```shell
   # kubectl describe cluster beijing
   Name:         beijing
   Namespace:    
   Kind:         Cluster
   Spec:
     Token:   eyJhbGciOi--omitted--4PebW68A
   
   ```

   > Note: Within validity period, the token is used to initialize the member cluster.

3. Deploy the FabEdge in a member cluster using the token generated in the first step. The operator of the member cluster reports the connector information of the cluster to the host cluster .

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

