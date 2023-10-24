# Quickstart for FabEdge v0.5.0

[toc]

## Terminology

-  **Cloud Cluster**：a standard k8s cluster, located at the cloud side, providing the cloud computing capability.
-  **Edge Cluster**: a standard k8s cluster, located at the edge side, providing the edge computing capability.
-  **Edge Node**:  a k8s node, located at the edge side, joining the cloud cluster using the framework, such as KubeEdge.
-  **Host Cluster**:  a selective cloud cluster, used to manage the cross-cluster communication. The 1st cluster deployed by FabEdge must be host cluster.
-  **Member Cluster**: an edge cluster, registered into the host cluster,  reports the network information to host cluster. 
-  **Community**：K8S CRD defined by FabEdge，there are two types： 
   - **Node Type**： to define the communication between nodes within the same cluster
   - **Cluster Type**：to define the cross-cluster communication

## Prerequisite

- Kubernetes (v1.18.8，1.22.7)
- Flannel (v0.14.0) or Calico (v3.16.5)
- KubeEdge （v1.5）or SuperEdge（v0.5.0）or OpenYurt（ v0.4.1）

## Preparation

1. Make sure the following ports are allowed by firewall or security group. 
   - ESP(50)，UDP/500，UDP/4500
2. Collect the configuration of the current cluster

   ```shell
	$ curl -s http://116.62.127.76/installer/v0.5.0/get_cluster_info.sh | bash -
	This may take some time. Please wait.
	
	clusterDNS               : 169.254.25.10
	clusterDomain            : root-cluster
	cluster-cidr             : 10.233.64.0/18
	service-cluster-ip-range : 10.233.0.0/18
    ```

## Deploy FabEdge on the host cluster

1. Label **all** edge nodes

	```shell
	$ kubectl label node --overwrite=true edge1 node-role.kubernetes.io/edge=
	node/edge1 labeled
	$ kubectl label node --overwrite=true edge2 node-role.kubernetes.io/edge=
	node/edge2 labeled
	
	$ kubectl get no
	NAME     STATUS   ROLES    AGE   VERSION
	edge1    Ready    edge     22h   v1.18.2
	edge2    Ready    edge     22h   v1.18.2
	master   Ready    master   22h   v1.18.2
	node1    Ready    <none>   22h   v1.18.2
	```

2.  Deploy FabEdge   

	```shell
	$ curl 116.62.127.76/installer/v0.5.0/install.sh | bash -s -- --cluster-name beijing  --cluster-role host --cluster-zone beijing  --cluster-region china --connectors node1 --connector-public-addresses 10.22.46.47 --chart http://116.62.127.76/fabedge-0.5.0.tgz
	```
	> Note：  
	> **--connectors**: names of k8s nodes which connectors are located  
	> **--connector-public-addresses**:  ip addresses of k8s nodes which connectors are located  

3.  Verify the deployment  

	```shell
	$ kubectl get no
	NAME     STATUS   ROLES       AGE     VERSION
	edge1    Ready    edge        5h22m   v1.18.2
	edge2    Ready    edge        5h21m   v1.18.2
	master   Ready    master      5h29m   v1.18.2
	node1    Ready    connector   5h23m   v1.18.2
	
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
	nodelocaldns-fmx7f                        1/1     Running   0          17h
	nodelocaldns-kcz6b                        1/1     Running   0          17h
	nodelocaldns-pwpm4                        1/1     Running   0          17h
	
	$ kubectl get po -n fabedge
	NAME                                READY   STATUS    RESTARTS   AGE
	fabdns-7b768d44b7-bg5h5             1/1     Running   0          9m19s
	fabedge-agent-edge1                 2/2     Running   0          8m18s
	fabedge-cloud-agent-hxjtb           1/1     Running   4          9m19s
	fabedge-connector-8c949c5bc-7225c   2/2     Running   0          8m18s
	fabedge-operator-dddd999f8-2p6zn    1/1     Running   0          9m19s
	service-hub-74d5fcc9c9-f5t8f        1/1     Running   0          9m19s
	```

4.  Create community for edges which need to communicate with each other

	```shell
	$ cat > node-community.yaml << EOF
	apiVersion: fabedge.io/v1alpha1
	kind: Community
	metadata:
	  name: beijing-edge-nodes  # community name
	spec:
	  members:
	    - beijing.edge1    # format：{cluster name}.{edge node name}
	    - beijing.edge2  
	EOF
	
	$ kubectl apply -f node-community.yaml
	```

5.  Update the [edge computing framework](#%E5%92%8C%E8%BE%B9%E7%BC%98%E8%AE%A1%E7%AE%97%E6%A1%86%E6%9E%B6%E7%9B%B8%E5%85%B3%E7%9A%84%E9%85%8D%E7%BD%AE) dependent configuration
5.  Update the [CNI](#%E5%92%8CCNI%E7%9B%B8%E5%85%B3%E7%9A%84%E9%85%8D%E7%BD%AE) dependent configuration

## Deploy FabEdge in the member cluster

If any member cluster,  register it in the host cluster first, then deploy FabEdge in it.

1.  in the **host cluster**，create an edge cluster named "shanghai". Get the token for registration.  
	
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

2. Label **all** edge nodes

	```shell
	$ kubectl label node --overwrite=true edge1 node-role.kubernetes.io/edge=
	node/edge1 labeled
	$ kubectl label node --overwrite=true edge2 node-role.kubernetes.io/edge=
	node/edge2 labeled
	
	$ kubectl get no
	NAME     STATUS   ROLES    AGE   VERSION
	edge1    Ready    edge     22h   v1.18.2
	edge2    Ready    edge     22h   v1.18.2
	master   Ready    master   22h   v1.18.2
	node1    Ready    <none>   22h   v1.18.2
	```

3. Deploy FabEdge in the member cluster
	
	```shell
	curl 116.62.127.76/installer/v0.5.0/install.sh | bash -s -- --cluster-name shanghai --cluster-role member --cluster-zone shanghai  --cluster-region china --connectors node1 --chart http://116.62.127.76/fabedge-0.5.0.tgz --server-serviceHub-api-server https://10.22.46.47:30304 --host-operator-api-server https://10.22.46.47:30303 --connector-public-addresses 10.22.46.26 --init-token eyJ------omitted-----9u0
	```
	> Note：  
	> **--server-serviceHub-api-server**: endpoint of serviceHub in the host cluster  
	> **--host-operator-api-server**: endpoint of operator-api in the host cluster    
	> **--connector-public-addresses**: ip address of k8s nodes on which connectors are located in the member cluster  
	> **--init-token**: token when the member cluster is added in the host cluster  

4. Verify the deployment

	```shell
	$ kubectl get no
	NAME     STATUS   ROLES       AGE     VERSION
	edge1    Ready    edge        5h22m   v1.18.2
	edge2    Ready    edge        5h21m   v1.18.2
	master   Ready    master      5h29m   v1.18.2
	node1    Ready    connector   5h23m   v1.18.2
	
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
	nodelocaldns-fmx7f                        1/1     Running   0          17h
	nodelocaldns-kcz6b                        1/1     Running   0          17h
	nodelocaldns-pwpm4                        1/1     Running   0          17h
	
	$ kubectl get po -n fabedge
	NAME                                READY   STATUS    RESTARTS   AGE
	fabdns-7b768d44b7-bg5h5             1/1     Running   0          9m19s
	fabedge-agent-edge1                 2/2     Running   0          8m18s
	fabedge-cloud-agent-hxjtb           1/1     Running   4          9m19s
	fabedge-connector-8c949c5bc-7225c   2/2     Running   0          8m18s
	fabedge-operator-dddd999f8-2p6zn    1/1     Running   0          9m19s
	service-hub-74d5fcc9c9-f5t8f        1/1     Running   0          9m19s
	```
	
## Enable multi-cluster communication

1.  in the **host cluster**，create a community for all clusters which need to communicate with each other  

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
the DNS components need to be modified

- if `nodelocaldns` is used，modify `nodelocaldns` only,  
- if SuperEdge `edge-coredns` is used，modify `coredns` and `edge-coredns`,  
- modify `coredns` for others  

1.  Update `nodelocaldns`  

	```shell
	$ kubectl -n kube-system edit cm nodelocaldns
	global:53 {
	        errors
	        cache 30
	        reload
	        bind 169.254.25.10                 # local bind address
	        forward . 10.233.12.205            # cluset-ip of fab-dns service
	    }
	```

2.  Update `edge-coredns`  

	```shell
	$ kubectl -n edge-system edit cm edge-coredns
	global {
	   forward . 10.244.51.126                 # cluset-ip of fab-dns service
	}
	```

3.  Update `coredns `

	```shell
	$ kubectl -n kube-system edit cm coredns
	global {
	   forward . 10.109.72.43                 # cluset-ip of fab-dns service
	}
	```
	
4. Reboot coredns，edge-coredns or nodelocaldns to take effect


## Edge computing framework depend configuration
### for KubeEdge

1.  Make sure `nodelocaldns` is running on all edge nodes  

	```shell
	$ kubectl get po -n kube-system -o wide | grep nodelocaldns
	nodelocaldns-cz5h2                        1/1     Running   0          56m   10.22.46.47   master   <none>           <none>
	nodelocaldns-nk26g                        1/1     Running   0          47m   10.22.46.23   edge1    <none>           <none>
	nodelocaldns-wqpbw                        1/1     Running   0          17m   10.22.46.20   node1    <none>           <none>
	```

2.  Update `edgecore` for all edge nodes   

	```shell
	$ vi /etc/kubeedge/config/edgecore.yaml
	
	# edgeMesh must be disabled
	edgeMesh:
	  enable: false
	
	edged:
	    enable: true
	    cniBinDir: /opt/cni/bin
	    cniCacheDirs: /var/lib/cni/cache
	    cniConfDir: /etc/cni/net.d
	    networkPluginName: cni
	    networkPluginMTU: 1500   
	    clusterDNS: 169.254.25.10        # clusterDNS of get_cluster_info script output
	    clusterDomain: "root-cluster"    # clusterDomain of get_cluster_info script output
	```
	> **clusterDNS**：if no nodelocaldns，coredns service can be used.

3.  Reboot `edgecore` on all edge nodes  

	```shell
	$ systemctl restart edgecore
	```

### for SuperEdge

1.  Verify the service，if not ready，to rebuild the Pod 

	```shell
	$ kubectl get po -n edge-system
	application-grid-controller-84d64b86f9-29svc   1/1     Running   0          15h
	application-grid-wrapper-master-pvkv8          1/1     Running   0          15h
	application-grid-wrapper-node-dqxwv            1/1     Running   0          15h
	application-grid-wrapper-node-njzth            1/1     Running   0          15h
	edge-coredns-edge1-5758f9df57-r27nf            0/1     Running   8          15h
	edge-coredns-edge2-84fd9cfd98-79hzp            0/1     Running   8          15h
	edge-coredns-master-f8bf9975c-77nds            1/1     Running   0          15h
	edge-health-7h29k                              1/1     Running   3          15h
	edge-health-admission-86c5c6dd6-r65r5          1/1     Running   0          15h
	edge-health-wcptf                              1/1     Running   3          15h
	tunnel-cloud-6557fcdd67-v9h96                  1/1     Running   1          15h
	tunnel-coredns-7d8b48c7ff-hhc29                1/1     Running   0          15h
	tunnel-edge-dtb9j                              1/1     Running   0          15h
	tunnel-edge-zxfn6                              1/1     Running   0          15h
	
	$ kubectl delete po -n edge-system edge-coredns-edge1-5758f9df57-r27nf
	pod "edge-coredns-edge1-5758f9df57-r27nf" deleted
	
	$ kubectl delete po -n edge-system edge-coredns-edge2-84fd9cfd98-79hzp
	pod "edge-coredns-edge2-84fd9cfd98-79hzp" deleted
	```

2.  By default the master node has the taint of `node-role.kubernetes.io/master:NoSchedule`，which prevents fabedge-cloud-agent to start. It caused pods on the master node cannot communicate with the other Pods on the other nodes. If needed,  to modify the DamonSet of fabedge-cloud-agent to tolerant this taint。 

## CNI-dependent Configurations

### for Calico

Regardless the cluster role, add all Pod and Service network segments of all other clusters to the cluster with Calico, which prevents Calico from doing source address translation.  

one example with the clusters of:  host (Calico)  + member1 (Calico) + member2 (Flannel)

* on the host (Calico) cluster, to add the addresses of the member (Calico) cluster and the member(Flannel) cluster
* on the member1 (Calico) cluster, to add the addresses of the host (Calico) cluster and the member(Flannel) cluster
* on the member2 (Flannel) cluster, there is NO any configuration required. 

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

## FAQ

1. If asymmetric routes exist, to disable **rp_filter** on all cloud node  
 
   ```shell
   $ sudo for i in /proc/sys/net/ipv4/conf/*/rp_filter; do  echo 0 >$i; done 
   # save the configuration.
   $ sudo vi /etc/sysctl.conf
   net.ipv4.conf.default.rp_filter=0
   net.ipv4.conf.all.rp_filter=0
   ```

1. If Error with：“Error: cannot re-use a name that is still in use”.   to uninstall fabedge and try again.
   
   ```shell
   $ helm uninstall -n fabedge fabedge
   release "fabedge" uninstalled
   ```
