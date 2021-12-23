<<<<<<< HEAD
# FabEdge User manual

[toc]

English | [中文](user-guide_zh.md)

## Use the community

By default, the pod on the edge node can only access the cloud pod and the node, and the pod on the edge node cannot communicate with each other, in order to avoid unnecessary waste caused by the establishment of too many tunnels on the edge node. In order to make the edge nodes that need to communicate accessible to each other, we put forward the concept of community. When several edge nodes need to communicate with each other, a community can be established, and the nodes that need to communicate can be put into the list of community members, so that these community members can access each other.  

After multi-cluster communication is implemented, communities can also be used to organize clusters that need to communicate with each other.  

Creating a community is very simple. Suppose we now have an edge cluster, which we call "beijing" when deployed, and there are three edge nodes edge1, edge2, and edge3 in the cluster.  

Create the following community:  
=======
# FabEdge用户手册

[toc]

## 使用社区

在默认情况下，边缘节点的Pod只能访问云端Pod和节点，边缘节点上的Pod之间不能互通，这是为了避免边缘节点建立太多隧道造成不必要的浪费。为了使需要通信的边缘节点可以相互访问，我们提出了社区这个概念，当几个边缘节点需要相互通信时，可以建立一个社区，把需要通信的节点放入社区成员列表，那么这些社区成员就可以相互访问了。

在多集群通信实现后，社区也可以用来组织需要相互通信的集群。

创建一个社区非常简单，假设我们现在有一个边缘集群，部署时为集群命名为beijing，集群里有3个边缘节点edge1, edge2, edge3，为了使三者可以相互访问，
创建如下社区:
>>>>>>> 12553d51b3bdbcca1706d62d093ea79656cdfe49

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

<<<<<<< HEAD
> Note: The endpoint_name of a node is "cluster_name. node_name ". 

If we now want to communicate between "beijing"cluster and "shanghai" cluster ,we can create the following cluster:  
=======
_注: 社区成员的名字不是节点名称，而是端点名，一个节点的端点名"集群名.节点名"这样的格式生成的。_

假设我们还有另外一个边缘集群，部署时为集群命名为shanghai，我们现在需要将beijing和shanghai两个集群通信，创建如下集群:
>>>>>>> 12553d51b3bdbcca1706d62d093ea79656cdfe49

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

<<<<<<< HEAD
> Note: The member name is "cluster_name.connector".

## Register the cluster

Multi-cluster communication requires us to register endpoint information of each cluster in the host cluster:

1. Create a cluster resource in the host cluster: 
=======
*注: 跨集群通信主要是由connector实现，所以成员名称是各个集群的connector的端点名*



## 注册集群

多集群通信需要把各个集群的端点信息在主机群注册：

1. 在主集群创建一个cluster资源:
>>>>>>> 12553d51b3bdbcca1706d62d093ea79656cdfe49

   ```yaml
   apiVersion: fabedge.io/v1alpha1
   kind: Cluster
   metadata:
     name: beijing
   ```

<<<<<<< HEAD
2. Check the token
=======
2. 查看token
>>>>>>> 12553d51b3bdbcca1706d62d093ea79656cdfe49

   ```shell
   # kubectl describe cluster beijing
   Name:         beijing
   Namespace:    
   Kind:         Cluster
   Spec:
<<<<<<< HEAD
     Token:   eyJhbGciOi--omitted--4PebW68A
   
   ```

   > Note: Within validity period, the token is used to initialize the member cluster.

3. Deploy the FabEdge in a member cluster using the token generated in the first step. The operator of the member cluster reports the connector information of the cluster to the host cluster .
=======
     Token:   eyJhbGciOi--省略--4PebW68A
   ```
   
   *注: token由fabedge-operator负责生成，该token有效期内使用该token进行成员集群初始化*
   
3. 在成员集群部署FabEdge，部署时使用第一步生成的token, 成员集群的operator会把本集群的connector信息上报至主机群。
>>>>>>> 12553d51b3bdbcca1706d62d093ea79656cdfe49

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
<<<<<<< HEAD
     token: eyJhbGciOi--omit--4PebW68A
   ```

=======
     token: eyJhbGciOi--省略--4PebW68A
   ```
>>>>>>> 12553d51b3bdbcca1706d62d093ea79656cdfe49
