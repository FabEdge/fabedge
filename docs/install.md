# FabEdge Installation Guide

English | [中文](install_zh.md)

[toc]

## Concepts

- **Cloud cluster**: a standard K8S cluster located in the cloud, providing the cloud part services.
- **Edge cluster**: a standard K8S cluster located on the edge to provide edge part services.

- **Host cluster**：a cloud cluster which manages the communication of member clusters. The first cluster deployed by FabEdge must be the host cluster.
- **Member cluster**：an edge cluster that registers with the host cluster and reports the endpoint network configuration of the cluster for multi-cluster communication.
- **Edge node** : a node is joined into the cloud cluster to provide edge part services using edge computing framework such as KubeEdge.

- **Community** : CRD defined by FabEdge,  used in two scenarios.
  - Defines the communication between edge nodes
  - Defines the communication between member clusters



## Prerequisites

- Kubernetes (v1.18.8)
- Flannel (v0.14.0) or Calico (v3.16.5)



## Preparations

1. Make sure the the firewall or security group allows the following protocols and ports.

   - ESP(50)，UDP/500，UDP/4500

1. Install helm for each cluster

     ```shell
     $ wget https://get.helm.sh/helm-v3.6.3-linux-amd64.tar.gz
     $ tar -xf helm-v3.6.3-linux-amd64.tar.gz
     $ cp linux-amd64/helm /usr/bin/helm 
     ```
     



## Deploying FabEdge in the host cluster 

1. Get the cluster information for future use

   ```shell
   $ curl -s http://116.62.127.76/get_cluster_info.sh | bash -
   This may take some time. Please wait.
   
   clusterDNS               : 169.254.25.10
   clusterDomain            : root-cluster
   cluster-cidr             : 10.233.64.0/18
   service-cluster-ip-range : 10.233.0.0/18
   ```

2. To label all edge node with `node-role.kubernetes.io/edge`

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
   
3. On the master node, modify the CNI DaemonSet and prevent it from running on the edge nodes. 

   ```bash
   $ cat > cni-ds.patch.yaml << EOF
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
   
   # If using Flannel
   $ kubectl patch ds -n kube-system kube-flannel-ds --patch "$(cat cni-ds.patch.yaml)"
   
   # If using Calico
   $ kubectl patch ds -n kube-system calico-node --patch "$(cat cni-ds.patch.yaml)"
   ```

4. Verify there is **no** any CNI component running on the any edge nodes

   ```shell
   $ kubectl get po -n kube-system -o wide | egrep -i "flannel|calico"
   calico-kube-controllers-8b5ff5d58-d2pkj   1/1     Running   0          67m   10.20.8.20    master
   calico-node-t5vww                         1/1     Running   0          38s   10.20.8.28    node1
   calico-node-z2fmf                         1/1     Running   0          62s   10.20.8.20    master
   ```

5. Select one or two nodes to run connector and label it with `node-role.kubernetes.io/connector`

   ```shell
   $ kubectl label no node1 node-role.kubernetes.io/connector=
   $ kubectl get node
   NAME     STATUS   ROLES       AGE     VERSION
   edge1    Ready    edge        5h22m   v1.18.2
   edge2    Ready    edge        5h21m   v1.18.2
   master   Ready    master      5h29m   v1.18.2
   node1    Ready    connector   5h23m   v1.18.2
   ```

6. Prepare the values.yaml file.

   ```shell
   $ cat > values.yaml << EOF
   operator:
     # edgePodCIDR: 10.10.0.0/16 
     connectorPublicAddresses:
     - 10.20.8.28
     serviceClusterIPRanges:
     - 10.233.0.0/18
       
     cluster:
       name: fabedge
       role: host
     
     operatorAPIServer:
       nodePort: 30303
   
   EOF
   ```
   
   > Note：
   >
   > **edgePodCIDR**：it must be set, if use calico; it can not be set if Flannel is used.
   >
   > **connectorPublicAddresses**:  The ip address of the selected node to run the `connector` service.  Make sure all edge nodes can reach it.
   >
   > **serviceClusterIPRanges**:  the network segment used by services in cloud cluster,  the output of `service_cluster_ip_range` in `get_cluster_info` script. 
   >
   > **cluster**: Configure the cluster name and the cluster role, the cluster name must be unique, the first cluster in FabEdge scope must be host cluster.  
   >
   > **operatorAPIServer**: Configures the `NodePort` of the `operator api server` service
   
7. Deploy Fabedge.

   ```
   $ helm install fabedge --create-namespace -n fabedge -f values.yaml http://116.62.127.76/fabedge-0.4.0.tgz
   ```

8. Verify that services are running on the master node

   ```shell
   # Verify all nodes are ready.
   $ kubectl get no
   NAME     STATUS   ROLES       AGE     VERSION
   edge1    Ready    edge        5h22m   v1.18.2
   edge2    Ready    edge        5h21m   v1.18.2
   master   Ready    master      5h29m   v1.18.2
   node1    Ready    connector   5h23m   v1.18.2
   
   # Verify Kubernetes service is normal.
   $ kubectl get po -n kube-system
   NAME                                       READY   STATUS    RESTARTS   AGE
   controlplane-master                        4/4     Running   0          159m
   coredns-546565776c-44xnj                   1/1     Running   0          159m
   coredns-546565776c-7vvnl                   1/1     Running   0          159m
   kube-flannel-ds-hbb7j                      1/1     Running   0          28m
   kube-flannel-ds-zmwbd                      1/1     Running   0          28m
   kube-proxy-47c5j                           1/1     Running   0          153m
   kube-proxy-4fckj                           1/1     Running   0          152m
   
   # Verify FabEdge service is normal.
   $ kubectl get po -n fabedge
   NAME                               READY   STATUS    RESTARTS   AGE
   connector-5947d5f66-hnfbv          2/2     Running   0          35m
   fabedge-agent-edge1                2/2     Running   0          22s
   fabedge-operator-dbc94c45c-r7n8g   1/1     Running   0          55s
   
   ```
   
9. Add all edge nodes which need to communicate directly into one `community`.

    ```shell
    $ cat > node-community.yaml << EOF
    apiVersion: fabedge.io/v1alpha1
    kind: Community
    metadata:
      name: connectors
    spec:
      members:
        - fabedge.edge1
        - fabedge.edge2  
    EOF
    
    $ kubectl apply -f node-community.yaml
    ```
1. Modify [framework-dependent configuration](#framework-dependent-configuration) 
1. Modify [CNI-dependent configuration](#cni-dependent-configurations)


## Deploy FabEdge in the member cluster (optional)

1. In the host cluster, add a member cluster named "beijing" for example and get the token for regestration. 

   ```shell
   # run on the master node in the host cluster.
   $ cat > beijing.yaml << EOF
   apiVersion: fabedge.io/v1alpha1
   kind: Cluster
   metadata:
     name: beijing
   EOF
   
   $ kubectl apply -f beijing.yaml
   
   $ kubectl get cluster beijing -o go-template --template='{{.spec.token}}' | awk 'END{print}' 
   eyJ------omitted-----9u0
   ```

7. In the local member cluster, prepare the helm values.yaml file 

   ```shell
   # run on master
   $ cat > values.yaml << EOF
   operator:
     # edgePodCIDR: 10.10.0.0/16 
     connectorPublicAddresses:
     - 10.20.8.12
     serviceClusterIPRanges:
     - 10.234.0.0/18
   
     cluster:
       name: beijing 
       role: member    # must be "member"
     
     hostOperatorAPIServer: https://10.20.8.28:30303  # the url of the Operator api server in host cluster 
     
     initToken: eyJ------omitted-----9u0
   
   EOF
   ```
   
8.  The others are same as in the host cluster.  See the previous section for details.




## Create a multi-cluster Community (optional）

1. Add the clusters that need to communicate into one community.

   ```shell
   # run on the host cluster
   $ cat > community.yaml << EOF
   apiVersion: fabedge.io/v1alpha1
   kind: Community
   metadata:
     name: connectors
   spec:
     members:
       - fabedge.connector  #{cluster_name}.connector
       - beijing.connector    
   EOF
   
   $ kubectl apply -f community.yaml
   ```
   



## framework-dependent Configuration

### If KubeEdge is used

1. Make sure `nodelocaldns` is running on **all edge nodes**

   ```
   $ kubectl get po -n kube-system -o wide | grep nodelocaldns
   nodelocaldns-ckpb4                              1/1     Running        1          6d5h    10.22.46.15     node1    <none>           <none>
   nodelocaldns-drmlz                              0/1     Running        0          2m50s   10.22.46.40     edge1   <none>           <none>
   nodelocaldns-vbxf9                              1/1     Running        1          4h6m    10.22.46.23     master   <none>           <none>
   
2. Modify edgecore configuration on **all edge nodes**.

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
       clusterDNS: 169.254.25.10      #  clusterDNS of get_cluster_info script output    
       clusterDomain: root-cluster  # clusterDomain of get_cluster_info script output
   ```

   > clusterDNS: if `nodelocaldns` is not enabled,  please use the service ip of `cordons`

3. Restart edgecore on **each edge node**.

   ```shell
   $ systemctl restart edgecore
   ```

### If SuperEdge is used

1. Check the pod status. If  any pod is not ready,  to delete and rebuild it. 

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

1. The pod on the master node cannot communicate with edge pod.

    SupeEdge has a taint of `node-role.kubernetes.io/master:NoSchedule` on master node by default, so fabedge-cloud-agent is not running on it. If needed, to update the DaemonSet of the fabedge-cloud-agent to tolerate it.  



## CNI-dependent Configurations

### If Calico is used

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
