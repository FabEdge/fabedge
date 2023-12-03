# 手动安装

本文展示如何通过在不使用`quickstart.sh`脚本的情况如何安装FabEdge。文章里的环境是kubeedge+Calico组合，集群为主机群，部分配置和参数可能不适合您的环境，请根据需求调整，

*注： 有关边缘框架，DNS配置注意事项请参考[快速安装](./get-started_zh.md)，本文不再赘述。*

## 前提条件

- Kubernetes (v1.22.5+)

- Flannel (v0.14.0) 或者 Calico (v3.16.5)

- KubeEdge （>= v1.9.0）或者 SuperEdge（v0.8.0）或者 OpenYurt（ >= v1.2.0）

- Helm3


## 安装FabEdge

1. 确保防火墙或安全组允许以下协议和端口 
   - ESP(50)，UDP/500，UDP/4500
   
2. 获取集群配置信息，供后面使用  
	
	```shell
	$ curl -s https://fabedge.github.io/helm-chart/scripts/get_cluster_info.sh | bash -
	This may take some time. Please wait.
		
	clusterDNS               : 169.254.25.10
	clusterDomain            : cluster.local
	cluster-cidr             : 10.233.64.0/18
	service-cluster-ip-range : 10.233.0.0/18
	```

3. 为Connector节点打标签

	```shell
	$ kubectl label node --overwrite=true node1 node-role.kubernetes.io/connector=
	node/node1 labeled
	
	$ kubectl get no node1
	NAME     STATUS   ROLES     AGE   VERSION
	node1    Ready    connector 22h   v1.18.2
	```

4. 为边缘节点打标签(新添加的节点也需要)：

	```shell
	$ kubectl label node --overwrite=true edge1 node-role.kubernetes.io/edge=
	node/edge1 labeled
	$ kubectl label node --overwrite=true edge2 node-role.kubernetes.io/edge=
	node/edge2 labeled
	
	$ kubectl get no
	NAME     STATUS   ROLES      AGE   VERSION
	edge1    Ready    edge        5h22m   v1.22.6-kubeedge-v1.12.2
	edge2    Ready    edge        5h21m   v1.22.6-kubeedge-v1.12.2
	master   Ready    master      5h29m   v1.22.5
	node1    Ready    connector   5h23m   v1.22.5
	```

5. 确保CNI组件不运行在边缘节点，这里以Calico为例

   ```yaml
   cat > /tmp/cni-ds.patch.yaml << EOF
   spec:
     template:
       spec:
         affinity:
           nodeAffinity:
             requiredDuringSchedulingIgnoredDuringExecution:
               nodeSelectorTerms:
               - matchExpressions:
                 - key: kubernetes.io/os
                   operator: In
                   values:
                   - linux
                 - key: node-role.kubernetes.io/edge
                   operator: DoesNotExist
   EOF
   kubectl patch ds -n kube-system calico-node --patch-file /tmp/cni-ds.patch.yaml
   ```

6. 用helm添加fabedge repo: 

   ```shell
   helm repo add fabedge https://fabedge.github.io/helm-chart
   ```

 7. 准备`values.yaml`

```yaml
cluster:
  name: beijing
  role: host
  region: beijing
  zone: beijing
  cniType: "calico"
  
  # 如果是flannel，可以不配置这个参数;
  # 另外这个参数需要注意不要跟当前集群的cluster-cidr参数重叠
  edgePodCIDR: "10.234.64.0/18" 
  # 填入步骤2中的cluster-cidr
  clusterCIDR: "10.233.64.0/18"
  connectorPublicAddresses:
  - 10.22.48.16
  # 通常connector需要被边缘节点的fabedge-agent访问需要映射端口，
  # 如果外部端口不能映射为500,需要修改该参数
  connectorPublicPort: 500
  # 是否使用connector节点作为mediator，如果边缘节点位于NAT网络后，
  # 彼此之间不能正常建立隧道，建议开启该功能
  connectorAsMediator: false
  # 填入步骤2中的service-cluster-ip-range
  serviceClusterIPRange:
  - 10.233.0.0/18

fabDNS:
  # 如果是多集群通信，并且需要多集群服务发现功能，需要设置为true 
  create: true 

agent:
  args:
    # 如果是superedge/openyurt环境，将以下参数设置为false; kubeedge环境下建议打开
    ENABLE_PROXY: "true" 
    ENABLE_DNS: "true" 
```

*注:  示例的`values.yaml`并非完整内容，完整的values文件可以通过执行`helm show values fabedge/fabedge`的方式获取。*

8. 安装FabEdge

   ```shell
   helm install fabedge fabedge/fabedge -n fabedge --create-namespace -f values.yaml
   ```

如果以下Pod运行正常，则安装成功

```shell
$ kubectl get po -n fabedge
NAME                                READY   STATUS    RESTARTS   AGE
fabdns-7b768d44b7-bg5h5             1/1     Running   0          9m19s
fabedge-agent-bvnvj                 2/2     Running   0          8m18s
fabedge-cloud-agent-hxjtb           1/1     Running   4          9m19s
fabedge-connector-8c949c5bc-7225c   2/2     Running   0          8m18s
fabedge-operator-dddd999f8-2p6zn    1/1     Running   0          9m19s
service-hub-74d5fcc9c9-f5t8f        1/1     Running   0          9m19s
```

*注：其中fabedge-connector, fabedge-operator必须存在，fabedge-agent-XXX只会运行在边缘节点， fabedge-cloud-agent只有非connector和非边缘节点才会存在， fabdns和service-hub在fabdns.create为true时才会安装。*
