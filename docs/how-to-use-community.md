## 如何使用社区

在默认情况下，边缘节点的Pod只能访问云端Pod和节点，边缘节点之间不能互通, 这是为了避免边缘节点建立太多隧道造成不必要的浪费。为了使需要通信的边缘节点可以相互访问，我们
提出了社区这个概念，在同一社区的边缘节点可以相互访问. 

创建一个社区非常简单，假设我们现在有一个边缘集群，集群里有3个边缘节点edge1, edge2, edge3，为了使三者可以相互访问，
创建如下社区:

```yaml
apiVersion: community.fabedge.io/v1alpha1
kind: Community
metadata:
  name: all-edge-nodes
spec:
  members:
    - edge1
    - edge2
    - edge3
```