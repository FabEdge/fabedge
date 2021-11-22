## 如何使用社区

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