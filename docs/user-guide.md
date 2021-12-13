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



## 注册集群

多集群通信需要把各个集群的端点信息在主机群注册：

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

3. 在成员集群部署FabEdge，部署时使用第一步生成的token, 成员集群的operator会把本集群的connector信息上报至主机群。

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



## 校验证书

FabEdge相关的证书，包括CA， Connector， Agent都保存在Secret里，Operator会自动维护这些证书。如果出现证书相关错误，可以使用下面方法手动验证。

```shell
# 在master节点上执行

# 启动一个cert的容器
docker run fabedge/cert

# 获取刚启动的容器的ID 
docker ps -a | grep cert
65ceb57d6656   fabedge/cert                  "/usr/local/bin/fabe…"   15 seconds ago   

# 将可执行程序拷贝到主机
docker cp 65ceb57d6656:/usr/local/bin/fabedge-cert .

# 查看相关Secret
kubectl get secret -n fabedge
NAME                            TYPE                                  DATA   AGE
api-server-tls                  kubernetes.io/tls                     4      3d22h
cert-token-csffn                kubernetes.io/service-account-token   3      3d22h
connector-tls                   kubernetes.io/tls                     4      3d22h
default-token-rq9mv             kubernetes.io/service-account-token   3      3d22h
fabedge-agent-tls-edge1         kubernetes.io/tls                     4      3d22h
fabedge-agent-tls-edge2         kubernetes.io/tls                     4      3d22h
fabedge-ca                      Opaque                                2      3d22h
fabedge-operator-token-tb8qb    kubernetes.io/service-account-token   3      3d22h

# 校验相关Secret
./fabedge-cert verify -s connector-tls
Your cert is ok
./fabedge-cert verify -s fabedge-agent-tls-edge1
Your cert is ok
```
