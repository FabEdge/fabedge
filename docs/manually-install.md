# Manually Install

This article will show you how to install FabEdge without `quickstart.sh`。The cluster uses KubeEdge and Calico and will be used as host cluster. Some settings may not suit your cases, you may need to change them according to your environment.

*PS: About how to configure edge frameworks and DNS, please checkout [Get Started](./get-started.md), We won't repeat it again.*

## Prerequisite

- Kubernetes (v1.22.5+)

- Flannel (v0.14.0) or Calico (v3.16.5)

- KubeEdge (>= v1.9.0) or SuperEdge(v0.8.0) or OpenYurt( >= v1.2.0)

- Helm3



## Deploy FabEdge

1. Make sure the following ports are allowed by firewall or security group. 
   - ESP(50)，UDP/500，UDP/4500
   
2. Collect the configuration of the current cluster  
	
	```shell
	$ curl -s https://fabedge.github.io/helm-chart/scripts/get_cluster_info.sh | bash -
	This may take some time. Please wait.
		
	clusterDNS               : 169.254.25.10
	clusterDomain            : cluster.local
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
	edge1    Ready    edge        5h22m   v1.22.6-kubeedge-v1.12.2
	edge2    Ready    edge        5h21m   v1.22.6-kubeedge-v1.12.2
	master   Ready    master      5h29m   v1.22.5
	node1    Ready    connector   5h23m   v1.22.5
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

6. Add fabedge repo using helm: 

   ```shell
   helm repo add fabedge https://fabedge.github.io/helm-chart
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
  # It's the value of "cluster-cidr" fetched in Step 2
  clusterCIDR: "10.233.64.0/18"
  # Usually connector should be accessible by fabedge-agent by port 500,
  # if you can't map public port 500, change this parameter.
  connectorPublicPort: 500
  # If your edge nodes are behind NAT networks and are hard to establish
  # tunnels between them, set this parameter to true, this will let connector
  # work also as a mediator to help edge nodes to establish tunnels.
  connectorAsMediator: false
  connectorPublicAddresses:
  - 10.22.48.16
  # It's the value of "service-cluster-ip-range" fetched in Step 2
  serviceClusterIPRange:
  - 10.234.0.0/18

fabDNS: 
  # If you need multi-cluster service discovery, set create to true
  create: true 

agent:
  args:
    # If your cluster uses superedge or openyurt, set them to false;
    # If your cluster uses kubeedge, it's better to set them to true
    ENABLE_PROXY: "true" 
    ENABLE_DNS: "true"
```

*P.S.: The code snippet above shows part of `values.yaml`, while you can get the complete `values.yaml` example by executing `helm show values fabedge/fabedge`.*

8. Deploy FabEdge

   ```shell
   helm install fabedge fabedge/fabedge -n fabedge --create-namespace -f values.yaml
   ```

If those pods following are running, you make it.

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

*PS： fabedge-connector and fabedge-operator are necessary, fabedge-agent-XXX will only run on edge nodes, fabedge-cloud-agent will only run non-connector and non-edge nodes, fabdns and service-hub will be installed only if fabdns.create is true*

