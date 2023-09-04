# Getting Started

[toc]

## Terminology

- **Cloud Cluster**:a standard k8s cluster, located at the cloud side, providing the cloud computing capability.
- **Edge Cluster**: a standard k8s cluster, located at the edge side, providing the edge computing capability.
- **Connector Node**: a k8s node, located at the cloud side,  connector is responsible for communication between the cloud side and edge side. Since a connector node will have a large traffic burden, it's better not to run other programs on them.
- **Edge Node**:  a k8s node, located at the edge side, joining the cloud cluster using the framework, such as KubeEdge.
- **Host Cluster**:  a selective cloud cluster, used to manage cross-cluster communication. The 1st cluster deployed by FabEdge must be the host cluster.
- **Member Cluster**: an edge cluster, registered into the host cluster,  reports the network information to the host cluster. 
- **Community**: an K8S CRD defined by FabEdge， there are two types:
   - **Node Type**: to define the communication between nodes within the same cluster
   - **Cluster Type**: to define the cross-cluster communication

## Prerequisite

- Kubernetes (v1.22.5+)
- Flannel (v0.14.0) or Calico (v3.16.5)
- KubeEdge (>= v1.9.0) or SuperEdge(v0.8.0) or OpenYurt(>= v1.2.0)

*PS1: For flannel, only Vxlan mode is supported. Support dual-stack environment.*

*PS2: For calico, only IPIP mode is supported. Support IPv4 environment only.*  

## Preparation

1. Make sure the following ports are allowed by the firewall or security group. 
   - ESP(50)，UDP/500，UDP/4500

2. Turn off firewalld if your machine has it.
   
3. Collect the configuration of the current cluster

   ```shell
	$ curl -s https://fabedge.github.io/helm-chart/scripts/get_cluster_info.sh | bash -
	This may take some time. Please wait.
	
	clusterDNS               : 169.254.25.10
	clusterDomain            : cluster.local
	cluster-cidr             : 10.233.64.0/18
	service-cluster-ip-range : 10.233.0.0/18
   ```

## Deploy FabEdge on the host cluster

1. Use helm to add fabedge repo:

   ```shell
   helm repo add fabedge https://fabedge.github.io/helm-chart
   ```
   
1. Deploy FabEdge   

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
   > Note:     
   > **--connectors**: The names of k8s nodes in which connectors are located, those nodes will be labeled as node-role.kubernetes.io/connector  
   > **--edges:** The names of edge nodes， those nodes will be labeled as node-role.kubernetes.io/edge  
   > **--edge-pod-cidr**: The range of IPv4 addresses for the edge pod, it is required if you use Calico. Please make sure the value is not overlapped with cluster CIDR of your cluster.  
   > **--connector-public-addresses**:  IP addresses of k8s nodes which connectors are located  

   *PS: The `quickstart.sh` script has more parameters， the example above only uses the necessary parameters, execute `quickstart.sh --help` to check all of them.*

2. Verify the deployment  

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
   
3. Create a community for edges that need to communicate with each other

   ```shell
   $ cat > all-edges.yaml << EOF
   apiVersion: fabedge.io/v1alpha1
   kind: Community
   metadata:
     name: beijing-edge-nodes  # community name
   spec:
     members:
       - beijing.edge1    # format:{cluster name}.{edge node name}
       - beijing.edge2  
   EOF
   
   $ kubectl apply -f all-edges.yaml
   ```

4. Update the [edge computing framework](#edge-computing-framework-dependent-configuration) dependent configuration

5. Update the [CNI](#cni-dependent-configurations) dependent configuration

## Deploy FabEdge in the member cluster

If you have any member cluster,  register it in the host cluster first, then deploy FabEdge in it. Before you that, you'd better to make sure none of the addresses of host network and container network of those clusters overlap.

1.  In the **host cluster**，create an edge cluster named "shanghai". Get the token for registration.  
	
	```shell
	# Run in the host cluster
	$ cat > shanghai.yaml << EOF
	apiVersion: fabedge.io/v1alpha1
	kind: Cluster
	metadata:
	  name: shanghai # cluster name
	EOF
	
	$ kubectl apply -f shanghai.yaml
	
	$ kubectl get cluster shanghai -o go-template --template='{{.spec.token}}' | awk 'END{print}' 
	eyJ------omitted-----9u0
	```

3. Use helm to add fabedge repo:
	
	```shell
	helm repo add fabedge https://fabedge.github.io/helm-chart
	```
	
3. Deploy FabEdge in the member cluster
	
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
	> Note:  
	> **--connectors**: The names of k8s nodes in which connectors are located, those nodes will be labeled as node-role.kubernetes.io/connector  
	> **--edges:** The names of edge nodes， those nodes will be labeled as node-role.kubernetes.io/edge  
	> **--edge-pod-cidr**: The range of IPv4 addresses for the edge pod, if you use Calico, this is required. Please make sure the value is not overlapped with cluster CIDR of your cluster.  
	> **--connector-public-addresses**: ip address of k8s nodes on which connectors are located in the member cluster  
	> **--init-token**: token when the member cluster is added in the host cluster  
	> **--service-hub-api-server**: endpoint of serviceHub in the host cluster  
	> **--operator-api-server**: endpoint of operator-api in the host cluster    
	
4. Verify the deployment

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
	fabdns-7b768d44b7-bg5h5             1/1     Running   0          9m19s
	fabedge-agent-m55h5                 2/2     Running   0          8m18s
	fabedge-cloud-agent-hxjtb           1/1     Running   4          9m19s
	fabedge-connector-8c949c5bc-7225c   2/2     Running   0          8m18s
	fabedge-operator-dddd999f8-2p6zn    1/1     Running   0          9m19s
	service-hub-74d5fcc9c9-f5t8f        1/1     Running   0          9m19s
	```
	
## Enable multi-cluster communication

1.  In the **host cluster**，create a community for all clusters which need to communicate with each other  

	```shell
	$ cat > community.yaml << EOF
	apiVersion: fabedge.io/v1alpha1
	kind: Community
	metadata:
	  name: all-clusters
	spec:
	  members:
	    - shanghai.connector   # format: {cluster name}.connector
	    - beijing.connector    # format: {cluster name}.connector
	EOF
	
	$ kubectl apply -f community.yaml
	```


## Enable multi-cluster service discovery
Change the coredns configmap:

```shell
$ kubectl -n kube-system edit cm coredns
# add this config
global {
   forward . 10.109.72.43                 # cluster-ip of fab-dns service
}

.:53 {
    ...
}
```

1.  Reboot coredns  to take effect


## Edge computing framework dependent configuration
### KubeEdge

#### cloudcore

1. Enable dynamicController of cloudcore:

   ```
   dynamicController:
       enable: true
   ```

   This configuration item is in the cloudcore configuration file cloudcore.yaml, please find the file yourself according to your environment.

2. Make sure cloudcore has permissions to access **endpointslices** resources (only if cloudcore is running in cluster):

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

3. Restart cloudcore.

#### edgecore

1. Update `edgecore` on all edge nodes   ( kubeedge < v.1.12.0)

   ```shell
   $ vi /etc/kubeedge/config/edgecore.yaml
   edged:
       enable: true
       ...
       networkPluginName: cni
       networkPluginMTU: 1500   
       clusterDNS: 169.254.25.10        
       clusterDomain: "cluster.local"    # clusterDomain from get_cluster_info script output
   metaManager:
       metaServer:
         enable: true
   ```

   or  ( kubeedge >= v.1.12.2)

   ```yaml
   $ vi /etc/kubeedge/config/edgecore.yaml
   edged:
       enable: true
       ...
       networkPluginName: cni
       networkPluginMTU: 1500 
       tailoredKubeletConfig:
           clusterDNS: ["169.254.25.10"]        
           clusterDomain: "cluster.local"   # clusterDomain from get_cluster_info script output  
   metaManager:
       metaServer:
         enable: true
   ```

3.  Reboot `edgecore` on all edge nodes  

	```shell
	$ systemctl restart edgecore 
	```

## CNI dependent Configurations

### for Calico

Since v0.7.0, fabedge can manage calico ippools of CIDRS from other clusters, the function is enabled when you use quickstart.sh to install fabedge. If you prefer to configure ippools by yourself, provide `--auto-keep-ippools false` when you install fabedge. If you choose to let fabedge configure ippools, the following content can be skipped.

Regardless of the cluster role, add all Pod and Service network segments of all other clusters to the cluster with Calico, which prevents Calico from doing source address translation.  

one example with the clusters of:  host (Calico)  + member1 (Calico) + member2 (Flannel)

* on the host (Calico) cluster, add the addresses of the member (Calico) cluster and the member(Flannel) cluster
* on the member1 (Calico) cluster, add the addresses of the host (Calico) cluster and the member(Flannel) cluster
* on the member2 (Flannel) cluster, there is no configuration required. 

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

> **cidr** should be one the of following values：
>
> * edge-pod-cidr of current cluster
> * cluster-cidr parameter of another cluster
> * service-cluster-ip-range of another cluster

## More Documents

* This document introduces how to install FabEdge via a script which help you to try FabEdge soon, but we would recommend you to read [Manually Install](./manually-install-zh.md) which might fit in production environment better.

* FabEdge also provide other features, read [FAQ](./FAQ.md) to find out.

  