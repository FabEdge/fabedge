# Manually Install

This article will show you how to install FabEdge without `quickstart.sh`。The cluster uses KubeEdge and Calico and will be used as host cluster. Some settings may not suit your cases, you may need to change them according to your enviroment.

*PS: About how to configure edge frameworks and DNS, please checkout [Get Started](./get-started.md), We won't repeat it again.*

## Prerequisite

- Kubernetes (v1.18.8，1.22.7)

- Flannel (v0.14.0) or Calico (v3.16.5)

- KubeEdge （v1.5）or SuperEdge（v0.5.0）or OpenYurt（ v0.4.1）

- Helm3

  


## Deploy FabEdge

1. Make sure the following ports are allowed by firewall or security group. 
   - ESP(50)，UDP/500，UDP/4500
   
2. Collect the configuration of the current cluster  
	
	```shell
	$ curl -s http://116.62.127.76/installer/v0.6.0/get_cluster_info.sh | bash -
	This may take some time. Please wait.
		
	clusterDNS               : 169.254.25.10
	clusterDomain            : root-cluster
	cluster-cidr             : 10.233.64.0/18
	service-cluster-ip-range : 10.233.0.0/18
	```

3. Label connector nodes:

	```shell
	$ kubectl label node --overwrite=true node1 node-role.kubernetes.io/connector=
	node/node1 labeled
	
	$ kubectl get no node1
	NAME     STATUS   ROLES     AGE   VERSION
	node1    Ready    connector 22h   v1.18.2
	```

4. Label all edge nodes: 

	```shell
	$ kubectl label node --overwrite=true edge1 node-role.kubernetes.io/edge=
	node/edge1 labeled
	$ kubectl label node --overwrite=true edge2 node-role.kubernetes.io/edge=
	node/edge2 labeled
	
	$ kubectl get no
	NAME     STATUS   ROLES      AGE   VERSION
	edge1    Ready    edge       22h   v1.18.2
	edge2    Ready    edge       22h   v1.18.2
	master   Ready    master     22h   v1.18.2
	node1    Ready    connector  22h   v1.18.2
	```

5. Make sure no CNI pods will run on edge nodes,  take Calico as an example: 

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

6. Download FabEdge chart: 

   ```shell
   wget http://116.62.127.76/fabedge-0.6.0.tgz
   ```

 7. Prepare your `values.yaml`

```yaml
cluster:
  name: beijing
  role: host
  region: beijing
  zone: beijing
  cniType: "calico"
  
  # edgePodCIDR is not necessary if your CNI is flannel;
  # Avoid an CIDR overlapped with cluster-cidr argument of your cluster
  edgePodCIDR: "10.234.64.0/18" 
  connectorPublicAddresses:
  - 10.22.48.16
  serviceClusterIPRange:
  - 10.234.0.0/18

fabDNS: 
  # If you need multi-cluster service discovery, set create to true
  create: true 

agent:
  args:
    # If your cluster uses superege or openyurt, set it to false;
    # If your cluster uses kubeedge, set it to true if you need it.
    ENABLE_PROXY: "true" 
```

*PS:  The `values.yaml` in the example is not complete, you can get the complete `values.yaml` example by executing `helm show values fabedge-0.6.0.tgz`.*

8. Deploy Fabedge

   ```shell
   helm install fabedge fabedge-0.6.0.tgz -n fabedge --create-namespace
   ```

If those pods following are running, you make it.

```shell
$ kubectl get po -n fabedge
NAME                                READY   STATUS    RESTARTS   AGE
fabdns-7b768d44b7-bg5h5             1/1     Running   0          9m19s
fabedge-agent-edge1                 2/2     Running   0          8m18s
fabedge-cloud-agent-hxjtb           1/1     Running   4          9m19s
fabedge-connector-8c949c5bc-7225c   2/2     Running   0          8m18s
fabedge-operator-dddd999f8-2p6zn    1/1     Running   0          9m19s
service-hub-74d5fcc9c9-f5t8f        1/1     Running   0          9m19s
```

*PS： fabedge-connector and fabedge-operator are necessary, fabedge-agent-XXX will only run on edge nodes, fabedge-cloud-agent will only run non-connector and non-edge nodes, fabdns and service-hub will be installed only if fabdns.create is true*

