# FabEdge用户手册

[toc]

## 网络管理

### 使用社区

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

### 自动组网

为了减少用户管理网络的负担，FabEdge提供了局域网自动组网的功能，自动组网会通过直连路由(direct routing)的方式让边缘Pod相互通信。要使用这个功能需要在安装时开启，具体的安装方式参考[手动安装](manually-install_zh.md)， 下面的配置文件供参考，请根据自己的环境调整：

```yaml
agent:
  args:
    AUTO_NETWORKING: "true" # 打开自动组网功能
    MULTICAST_TOKEN: "1b1bb567" # 组网信息令牌，请保证局域网内唯一，只有持有相同令牌的节点才能组网
    MULTICAST_ADDRESS: "239.40.20.81:18080" # fabedge-agent用来广播组网信息的地址
```

*注1： 自动组网仅限于同一路由器下的节点可用，当两个节点即在一个路由器下，又处于同一个社区，会优先使用自动组网功能。*

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
全局服务把本集群的一个普通的Service （Headless 或 ClusterIP），暴露给其它集群访问，并且提供基于拓扑的服务发现能力。  

1. 创建一个k8s的服务， 比如，命名空间是default， service的名字是web   

2. 为web服务添加标签：`fabedge.io/global-service: true`  

3. 所有集群可以通过域名：`web.default.svc.global`,  就近访问到web的服务。  

   更多内容请参考[如何创建全局服务](https://github.com/FabEdge/fab-dns/blob/main/docs/how-to-create-globalservice.md)及[示例](https://github.com/FabEdge/fab-dns/tree/main/examples)

## FabEdge Agent节点级参数配置

通常fabedge-agent的启动参数都是一致的，但fabedge允许您对特定节点的fabedge-agent指定参数，您仅需在节点的annotations配置fabedge-agent参数，fabedge-operator会自动更新相应的fabedge-agent pod。例如: 

```shell
kubectl annotate node edge1 argument.fabedge.io/enable-proxy=false # 关闭fab-proxy
```

每一个参数的格式都是"argument.fabedge.io/argument-name"，详细的参数列表参考[这里](https://github.com/FabEdge/fabedge/blob/main/pkg/agent/config.go#L63)

## 禁止在特定节点上运行fabedge-agent

fabedge-operator默认会为每一个边缘节点创建一个fabedge-agent pod，但fabedge允许您通过配置标签的方式来禁止fabedge-oeprator为指定节点创建fabedge-operator。首先您需要在安装fabedge修改边缘节点标签，具体安装方式参考[手动安装](manually-install_zh.md)，下面的配置文件供参考，请根据自己的环境调整：

```yaml
cluster:
  # fabedge-operator根据edgeLabels搜索边缘节点，您可以根据自己的需要修改以下内容
  edgeLabels:
  - node-role.kubernetes.io/edge=
  - agent.fabedge.io/enabled=true
```

假如您有两个边缘节点edge1与edge2，您仅需要edge1运行fabedge-agent，执行以下命令:

```yaml
kubectl label node edge1 node-role.kubernetes.io/edge=
kubectl label node edge1 agent.fabedge.io/enabled=true
```

就会只在edge1运行fabedge-agent。

