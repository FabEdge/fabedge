# FabEdge快速安装指南

[toc]

## 概念

-  **云端集群**：标准的K8S集群，位于云端，提供云端的计算能力 
-  **Connector节点**：标准的k8s节点，位于云端，负责云端和边缘端通信，因为可能会承载很多流量，尽量不要在该节点运行其他程序。
-  **边缘节点**：通过KubeEdge等边缘计算框架，加入云端集群的边缘侧节点，提供边缘计算能力 
-  **边缘集群**：标准的K8S集群，位于边缘侧，提供边缘计算能力
-  **主集群**：一个选定的云端集群，用于管理其它集群的跨集群通讯，FabEdge部署的第一个集群必须是主集群 
-  **成员集群**：一个边缘集群，注册到主集群，上报本集群端点网络配置信息用于多集群通讯 
-  **Community**：FabEdge定义的CRD，分为两类： 
   - **节点类型**：定义集群内多个边缘节点之间的通讯
   - **集群类型**：定义多个边缘集群之间的通讯

## 前提条件

- Kubernetes (v1.22.5+)

- Flannel (v0.14.0 ) 或者 Calico (v3.16.5)

- KubeEdge （>= v1.9.0）或者 SuperEdge（v0.8.0）或者 OpenYurt（ >= v1.2.0）

  *注1： Flannel目前仅支持Vxlan模式，支持双栈环境。*

  *注2： Calico目前仅支持IPIP模式，kube backend存储(默认)，不支持双栈环境。*

## 环境准备

1. 确保防火墙或安全组允许以下协议和端口 
   - ESP(50)，UDP/500，UDP/4500
   
2.  如果机器上有firewalld，也最好关闭
   
3. 获取集群配置信息，供后面使用  
	
	```shell
	$ curl -s https://fabedge.github.io/helm-chart/scripts/get_cluster_info.sh | bash -
	This may take some time. Please wait.
		
	clusterDNS               : 169.254.25.10
	clusterDomain            : cluster.local
	cluster-cidr             : 10.233.64.0/18
	service-cluster-ip-range : 10.233.0.0/18
	```

## 在主集群部署FabEdge

1. 用helm添加fabedge repo：

   ```shell
   helm repo add fabedge https://fabedge.github.io/helm-chart
   ```
   
1. 安装FabEdge   

   ```shell
   $ curl https://fabedge.github.io/helm-chart/scripts/quickstart.sh | bash -s -- \
   	--cluster-name beijing  \
   	--cluster-role host \
   	--cluster-zone beijing  \
   	--cluster-region china \
   	--connectors node1 \
   	--edges edge1,edge2 \
   	--edge-pod-cidr 10.233.0.0/16 \
   	--connector-public-addresses 10.22.46.47 \
   	--chart fabedge/fabedge
   ```
   > 说明：   
   > **--connectors**: connector所在节点主机名，指定的节点会被打上node-role.kubernetes.io/connector标签  
   > **--edges:** 边缘节点名称，指定的节点会被打上node-role.kubernetes.io/edge标签  
   > **--edge-pod-cidr**: 用来分配给边缘Pod的网段, 使用Calico时必须配置，并确保这个值不能跟集群的cluster-cidr参数重叠  
   > **--connector-public-addresses**: connector所在节点的公网IP地址，从边缘节点必须网络可达  
   
   *注：`quickstart.sh`脚本有很多参数，以上实例仅以最常用的参数举例，执行`quickstart.sh --help`查询。*
   
3.  确认部署正常  
	
	```shell
	$ kubectl get no
	NAME     STATUS   ROLES       AGE     VERSION
	edge1    Ready    edge        5h22m   v1.22.6-kubeedge-v1.12.2
	edge2    Ready    edge        5h21m   v1.22.6-kubeedge-v1.12.2
	master   Ready    master      5h29m   v1.22.5
	node1    Ready    connector   5h23m   v1.22.5
	
	$ kubectl get po -n kube-system
	NAME                                      READY   STATUS    RESTARTS   AGE
	calico-kube-controllers-8b5ff5d58-lqg66   1/1     Running   0          17h
	calico-node-7dkwj                         1/1     Running   0          16h
	calico-node-q95qp                         1/1     Running   0          16h
	coredns-86978d8c6f-qwv49                  1/1     Running   0          17h
	kube-apiserver-master                     1/1     Running   0          17h
	kube-controller-manager-master            1/1     Running   0          17h
	kube-proxy-ls9d7                          1/1     Running   0          17h
	kube-proxy-wj8j9                          1/1     Running   0          17h
	kube-scheduler-master                     1/1     Running   0          17h
	metrics-server-894c64767-f4bvr            2/2     Running   0          17h
	nginx-proxy-node1                         1/1     Running   0          17h
	
	$ kubectl get po -n fabedge
	NAME                                READY   STATUS    RESTARTS   AGE
	fabdns-7dd5ccf489-5dc29              1/1     Running   0             24h
	fabedge-agent-bvnvj                  2/2     Running   2 (23h ago)   24h
	fabedge-agent-c9bsx                  2/2     Running   2 (23h ago)   24h
	fabedge-cloud-agent-lgqkw            1/1     Running   3 (24h ago)   24h
	fabedge-connector-54c78b5444-9dkt6   2/2     Running   0             24h
	fabedge-operator-767bc6c58b-rk7mr    1/1     Running   0             24h
	service-hub-7fd4659b89-h522c         1/1     Running   0             24h
	```
	
4.  为需要通讯的边缘节点创建Community  
	
	```shell
	$ cat > all-edges.yaml << EOF
	apiVersion: fabedge.io/v1alpha1
	kind: Community
	metadata:
	  name: beijing-edge-nodes
	spec:
	  members:
	    - beijing.edge1
	    - beijing.edge2  
	EOF
	
	$ kubectl apply -f all-edges.yaml
	```

4. 根据使用的[边缘计算框架](#边缘计算框架相关的配置)修改相关配置 

5.  根据使用的[CNI](#CNI相关的配置)修改相关配置 

## 在成员集群部署FabEdge
如果有成员集群，先在主集群注册所有的成员集群，然后在每个成员集群部署FabEdge。在部署前，要注意确保各个集群的主机网络地址及容器网络地址不要重叠。

1. 在**主集群**添加一个名字叫“shanghai”的成员集群，获取Token供注册使用  

   ```shell
   $ cat > shanghai.yaml << EOF
   apiVersion: fabedge.io/v1alpha1
   kind: Cluster
   metadata:
     name: shanghai # 集群名字
   EOF
   
   $ kubectl apply -f shanghai.yaml
   
   $ kubectl get cluster shanghai -o go-template --template='{{.spec.token}}' | awk 'END{print}' 
   eyJ------省略内容-----9u0
   ```

3. 用helm添加fabedge repo：
	
	```shell
	helm repo add fabedge https://fabedge.github.io/helm-chart
	```
	
3. 在**成员集群**安装FabEdage  
	
	```shell
	curl https://fabedge.github.io/helm-chart/scripts/quickstart.sh | bash -s -- \
		--cluster-name shanghai \
		--cluster-role member \
		--cluster-zone shanghai  \
		--cluster-region china \
		--connectors node1 \
		--edges edge1,edge2 \
		--edge-pod-cidr 10.233.0.0/16 \
		--connector-public-addresses 10.22.46.26 \
		--chart fabedge/fabedge \
		--service-hub-api-server https://10.22.46.47:30304 \
		--operator-api-server https://10.22.46.47:30303 \
		--init-token ey...Jh
	```
	> 说明：  
	> **--connectors**: connector所在节点主机名，指定的节点会被打上node-role.kubernetes.io/connector标签    
	> **--edges:** 边缘节点名称，指定的节点会被打上node-role.kubernetes.io/edge标签  
	> **--edge-pod-cidr**: 用来分配给边缘Pod的网段, 使用Calico时必须配置。在v1.0.0版本前，这个值不能跟集群的cluster-cidr参数重叠，从v1.0.0起，建议该值是cluster-cidr的子集，但不要跟CALICO_IPV4POOL_CIDR/里的值重叠。   
	> **--connector-public-addresses**: member集群connectors所在节点的ip地址  
	> **--service-hub-api-server**: host集群serviceHub服务的地址和端口  
	> **--operator-api-server**: host集群operator-api服务的地址和端口  
	> **--init-token**: host集群获取的token  
	
4. 确认部署正常 
	
	```shell
	$ kubectl get no
	NAME     STATUS   ROLES       AGE     VERSION
	edge1    Ready    edge        5h22m   v1.22.6-kubeedge-v1.12.2
	edge2    Ready    edge        5h21m   v1.22.6-kubeedge-v1.12.2
	master   Ready    master      5h29m   v1.22.5
	node1    Ready    connector   5h23m   v1.22.5
	
	$ kubectl get po -n kube-system
	NAME                                      READY   STATUS    RESTARTS   AGE
	calico-kube-controllers-8b5ff5d58-lqg66   1/1     Running   0          17h
	calico-node-7dkwj                         1/1     Running   0          16h
	calico-node-q95qp                         1/1     Running   0          16h
	coredns-86978d8c6f-qwv49                  1/1     Running   0          17h
	kube-apiserver-master                     1/1     Running   0          17h
	kube-controller-manager-master            1/1     Running   0          17h
	kube-proxy-ls9d7                          1/1     Running   0          17h
	kube-proxy-wj8j9                          1/1     Running   0          17h
	kube-scheduler-master                     1/1     Running   0          17h
	metrics-server-894c64767-f4bvr            2/2     Running   0          17h
	
	$ kubectl get po -n fabedge
	NAME                                READY   STATUS    RESTARTS   AGE
	fabdns-7b768d44b7-bg5h5             1/1     Running   0          9m19s
	fabedge-agent-m55h5                 2/2     Running   0          8m18s
	fabedge-cloud-agent-hxjtb           1/1     Running   4          9m19s
	fabedge-connector-8c949c5bc-7225c   2/2     Running   0          8m18s
	fabedge-operator-dddd999f8-2p6zn    1/1     Running   0          9m19s
	service-hub-74d5fcc9c9-f5t8f        1/1     Running   0          9m19s
	```

## 启用多集群通讯

1.  在主集群，把所有须要通讯的集群加入一个Community  

	```shell
	# 在master节点操作
	$ cat > all-edges.yaml << EOF
	apiVersion: fabedge.io/v1alpha1
	kind: Community
	metadata:
	  name: all-edges
	spec:
	  members:
	    - shanghai.connector   # {集群名称}.connector
	    - beijing.connector    # {集群名称}.connector
	EOF
	
	$ kubectl apply -f all-edges.yaml
	```

## 启用多集群服务发现

修改集群的coredns配置：

```shell
$ kubectl -n kube-system edit cm coredns
# 添加该配置
global {
   forward . 10.109.72.43                 # fabdns的service IP地址
}

.:53 {
    ...
}
```


## 边缘计算框架相关的配置
### KubeEdge

#### cloudcore

1. 启动cloudcore的dynamicController:

   ```yaml
   dynamicController:
       enable: true
   ```

   该配置项在cloudcore的配置文件cloudcore.yaml中，请根据您的环境自行寻找该文件。

2. 确保cloudcore有访问endpointslices资源的权限(仅限于以Pod方式运行的cloudcore):

   ```
   kubectl edit clusterrole cloudcore
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     labels:
       app.kubernetes.io/managed-by: Helm
       k8s-app: kubeedge
       kubeedge: cloudcore
     name: cloudcore
   rules:
   - apiGroups:
     - discovery.k8s.io
     resources:
     - endpointslices
     verbs:
     - get
     - list
     - watch
   ```

3. 重启cloudcore

#### edgecore

1. 在**每个边缘节点**上修改edgecore配置 ( kubeedge < v.1.12.0)

   ```shell
   $ vi /etc/kubeedge/config/edgecore.yaml
   edged:
       enable: true
       ...
       networkPluginName: cni
       networkPluginMTU: 1500   
       clusterDNS: 169.254.25.10        
       clusterDomain: "cluster.local"    # get_cluster_info脚本输出的clusterDomain
   metaManager:
       metaServer:
         enable: true
   ```
   或者 ( kubeedge >= v.1.12.2)

   ```yaml
   $ vi /etc/kubeedge/config/edgecore.yaml
   edged:
       enable: true
       ...
       networkPluginName: cni
       networkPluginMTU: 1500 
       tailoredKubeletConfig:
           clusterDNS: ["169.254.25.10"]        
           clusterDomain: "cluster.local"    # get_cluster_info脚本输出的clusterDomain
   metaManager:
       metaServer:
         enable: true
   ```

2. 在**每个边缘节点**上重启edgecore  

   ```shell
   $ systemctl restart edgecore
   ```

## CNI相关的配置
### 如果使用Calico

自v0.7.0起，fabedge提供了自动维护calico ippools功能，使用`quickstart.sh`安装fabedge时，会自动启动这个功能。如果您希望自己管理calico ippools，可以在安装时使用`--auto-keep-ippools false`配置项关闭这个功能。在启用自动维护calico ippools的情况下，以下内容可以跳过。

不论是什么集群角色, 只要集群使用Calico，就要将本集群的EdgePodCIDR其它所有集群的Pod和Service的网段加入当前集群的Calico配置,  防止Calico做源地址转换，导致不能通讯。

例如: host (Calico)  + member1 (Calico) + member2 (Flannel)

- 在host (Calico) 集群的master节点操作，将member1 (Calico)，member2 (Flannel)地址配置到host集群的Calico中。
- 在member1 (Calico)集群的master节点操作，将host (Calico) ，member2 (Flannel)地址配置到member1集群的Calico中。
- 在member2 (Flannel)无需任何操作。

	```shell
	$ cat > cluster-cidr-pool.yaml << EOF
	apiVersion: projectcalico.org/v3
	kind: IPPool
	metadata:
	  name: cluster-beijing-cluster-cidr
	spec:
	  blockSize: 26
	  cidr: 10.233.64.0/18
	  natOutgoing: false
	  disabled: true
	  ipipMode: Always
	EOF
	
	$ calicoctl.sh create -f cluster-cidr-pool.yaml
	
	$ cat > service-cluster-ip-range-pool.yaml << EOF
	apiVersion: projectcalico.org/v3
	kind: IPPool
	metadata:
	  name: cluster-beijing-service-cluster-ip-range
	spec:
	  blockSize: 26
	  cidr: 10.233.0.0/18
	  natOutgoing: false
	  disabled: true
	  ipipMode: Always
	EOF
	
	$ calicoctl.sh create -f service-cluster-ip-range-pool.yaml
	```

> **cidr**参数是以下系统参数之一：
>
> * 本集群的edge-pod-cidr
> * 其他集群cluster-cidr
> * 其他集群的service-cluster-ip-range

## 更多资料

* 本文的安装方式是脚本安装，它让您能快速体验FabEdge，但建议您阅读[手动安装](./manually-install_zh.md)，这更适合在生产环境下的部署。
* FabEdge有许多特性，这些都记录在[常见问题](./FAQ_zh.md)。
* 如果您使用了多集群通信功能，建议您阅读[创建全局服务](https://github.com/FabEdge/fab-dns/blob/main/docs/how-to-create-globalservice.md)来知晓如何跨集群访问服务。

