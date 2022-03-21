# FabEdge用户手册

[toc]

## 使用社区

在默认情况下，边缘节点的Pod只能访问云端Pod和节点，边缘节点上的Pod之间不能互通，这是为了避免边缘节点建立太多隧道造成不必要的浪费。为了使需要通信的边缘节点可以相互访问，我们提出了社区这个概念，当几个边缘节点需要相互通信时，可以建立一个社区，把需要通信的节点放入社区成员列表，那么这些社区成员就可以相互访问了。

在多集群通信实现后，社区也可以用来组织需要相互通信的集群。

创建一个社区非常简单，假设我们现在有一个边缘集群，部署时为集群命名为beijing，集群里有3个边缘节点edge1, edge2, edge3，为了使三者可以相互访问，
创建如下社区:

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

_注: 社区成员的名字不是节点名称，而是端点名，一个节点的端点名"集群名.节点名"这样的格式生成的。_

假设我们还有另外一个边缘集群，部署时为集群命名为shanghai，我们现在需要将beijing和shanghai两个集群通信，创建如下集群:

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

*注: 跨集群通信主要是由connector实现，所以成员名称是各个集群的connector的端点名*



## 注册边缘集群

多集群通信需要把各个集群的端点信息在主集群注册：

1. 在主集群创建一个cluster资源:

   ```yaml
   apiVersion: fabedge.io/v1alpha1
   kind: Cluster
   metadata:
     name: beijing
  
   ```

2. 查看token

   ```shell
   # kubectl describe cluster beijing
   Name:         beijing
   Namespace:    
   Kind:         Cluster
   Spec:
     Token:   eyJhbGciOi--省略--4PebW68A
   ```
   
   *注: token由fabedge-operator负责生成，该token有效期内使用该token进行成员集群初始化*
   
3. 在成员集群部署FabEdge，部署时使用第一步生成的token, 成员集群的operator会把本集群的connector信息上报至主集群。

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
     token: eyJhbGciOi--省略--4PebW68A
   ```


## 为边缘节点指定公网地址

对于公有云的场景，云主机一般只配置了私有地址，导致FabEdge无法建立边缘到边缘的隧道。这种情况下可以为云主机申请一个公网地址，加入节点的注解，FabEdge将自动使用这个公网地址建立隧道，而不是私有地址。

```shell
# 为边缘节点edge1指定公网地址60.247.88.194
kubectl annotate node edge1 "fabedge.io/node-public-addresses=60.247.88.194"
```

## 创建全局服务
全局服务把本集群的一个普通的Service （Headless 或 ClusetrIP），暴露给其它集群访问，并且提供基于拓扑的服务发现能力。  

1. 创建一个k8s的服务， 比如，命名空间是default， service的名字是web   
2. 为web服务添加标签：`fabedge.io/global-service: true`  
3. 所有集群可以通过域名：`web.default.svc.global`,  就近访问到web的服务。  