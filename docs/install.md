# FabEdge installation and deployment

[toc]

English | [中文](install_zh.md)

## Conception

- Cloud cluster: a standard K8S cluster located in the cloud, providing cloud services.
- Edge node: Use edge computing framework such as KubeEdge to add a node to the cloud cluster and become one of its nodes to provide edge capabilities.
-  Edge cluster: a standard K8S cluster located on the edge to provide edge services.

Clusters be classified into two types based on roles:

- host cluster：a cloud cluster that manages communication between member clusters. The first cluster deployed by FabEdge must be the host cluster.
- member cluster：an edge cluster that registers with the host cluster and reports the endpoint network configuration of the cluster for multi-cluster communication.

Community： CRD defined by FabEdge, which can be used in two scenarios.

- Defines the communication between edge nodes in the cluster.
- Defines cross cluster communication.



## Precondition

- Kubernetes (v1.18.8)
- Flannel (v0.14.0) or Calico (v3.16.5)



## Environmental preparation

1. Make sure that the firewall or security group allows the following protocols and ports.

   - ESP(50)，UDP/500，UDP/4500

1. Install helm for each cluster.

     ```shell
     $ wget https://get.helm.sh/helm-v3.6.3-linux-amd64.tar.gz
     $ tar -xf helm-v3.6.3-linux-amd64.tar.gz
     $ cp linux-amd64/helm /usr/bin/helm 
     ```
     



## Deploying FabEdge in host cluster 

1. The current cluster configuration information is obtained for future use.

   ```shell
   $ curl -s http://116.62.127.76/get_cluster_info.sh | bash -
   This may take some time. Please wait.
   
   clusterDNS               : 169.254.25.10
   clusterDomain            : root-cluster
   cluster-cidr             : 10.233.64.0/18
   service-cluster-ip-range : 10.233.0.0/18
   ```

2. Label each edge node.

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
   
3. On the master node, modify the CNI DaemonSet and forbid running on the edge nodes. 

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

4. Verify that each edge nodes  are not running  any CNI components.

   ```shell
   $ kubectl get po -n kube-system -o wide | egrep -i "flannel|calico"
   calico-kube-controllers-8b5ff5d58-d2pkj   1/1     Running   0          67m   10.20.8.20    master
   calico-node-t5vww                         1/1     Running   0          38s   10.20.8.28    node1
   calico-node-z2fmf                         1/1     Running   0          62s   10.20.8.20    master
   ```

5. Select a node running connector in the cloud and label it. 

   ```shell
   $ kubectl label no node1 node-role.kubernetes.io/connector=
   $ kubectl get node
   NAME     STATUS   ROLES       AGE     VERSION
   edge1    Ready    edge        5h22m   v1.18.2
   edge2    Ready    edge        5h21m   v1.18.2
   master   Ready    master      5h29m   v1.18.2
   node1    Ready    connector   5h23m   v1.18.2
   ```

   > Note: Select nodes that allow normal PODS to run, and do not have the stain of not being able to schedule, otherwise the deployment will fail.  

6. Prepare values.yaml file.

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
   
   > Description：
   >
   > **edgePodCIDR**：If calico is used, it must be configured. If Flannel is used, this parameter cannot be configured.
   >
   > **connectorPublicAddresses**: The selected address of the node running the `connector` service to ensure that edge nodes can access it.  
   >
   > **serviceClusterIPRanges**: network segment used by services in cloud cluster, `service_cluster_ip_range` output by `get_cluster_info` script. 
   >
   > **cluster**: Configure the cluster name and the cluster role, the cluster name must not conflict, the first cluster must be the host cluster.  
   >
   > **operatorAPIServer**: Configures the `NodePort` of the `operator apiserver` component.  
   
7. Install Fabedge.

   ```
   $ helm install fabedge --create-namespace -n fabedge -f values.yaml http://116.62.127.76/fabedge-0.4.0.tgz
   ```

8. Verify that services are normal on **management node**.

   ```shell
   # Verify that nodes are ready.
   $ kubectl get no
   NAME     STATUS   ROLES       AGE     VERSION
   edge1    Ready    edge        5h22m   v1.18.2
   edge2    Ready    edge        5h21m   v1.18.2
   master   Ready    master      5h29m   v1.18.2
   node1    Ready    connector   5h23m   v1.18.2
   
   # Verify that the Kubernetes service is normal.
   $ kubectl get po -n kube-system
   NAME                                       READY   STATUS    RESTARTS   AGE
   controlplane-master                        4/4     Running   0          159m
   coredns-546565776c-44xnj                   1/1     Running   0          159m
   coredns-546565776c-7vvnl                   1/1     Running   0          159m
   kube-flannel-ds-hbb7j                      1/1     Running   0          28m
   kube-flannel-ds-zmwbd                      1/1     Running   0          28m
   kube-proxy-47c5j                           1/1     Running   0          153m
   kube-proxy-4fckj                           1/1     Running   0          152m
   
   # Verify that the FabEdge service is normal.
   $ kubectl get po -n fabedge
   NAME                               READY   STATUS    RESTARTS   AGE
   connector-5947d5f66-hnfbv          2/2     Running   0          35m
   fabedge-agent-edge1                2/2     Running   0          22s
   fabedge-operator-dbc94c45c-r7n8g   1/1     Running   0          55s
   
   ```
   
9. Add edge nodes that need to communicate directly to the same `community`.

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

   > In this example, edge nodes edge1 and edge2 of the cluster FabEdge are added to a same community to allow direct communication.



## Deploy FabEdge on a member cluster (optional)

1. Add a member cluster named "beijing" to get token for future registration.

   ```shell
   # Perform the operation on the master node in the host cluster.
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

2. Obtain the current cluster configuration information for future use.

   ```shell
   # Operate on the local member cluster.
   $ curl -s http://116.62.127.76/get_cluster_info.sh | bash -
   This may take some time. Please wait.
   
   clusterDNS               : 169.254.25.10
   clusterDomain            : root-cluster
   cluster-cidr             : 10.234.64.0/18
   service-cluster-ip-range : 10.234.0.0/18
   ```

3. Label all **edge nodes** 

   ```shell
   # Operate on the local member cluster.
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

4. On the master node, modify the **DaemonSet** of the existing **CNI** to prohibit running on the edge node.

   ```bash
   # Operate on the local member cluster.
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

5. Verify that **all edge nodes**  are not running any CNI components.

   ```shell
   $ kubectl get po -n kube-system -o wide | egrep -i "flannel|calico"
   kube-flannel-79l8h               1/1     Running   0          3d19h   10.20.8.24    master         
   kube-flannel-8j9bp               1/1     Running   0          3d19h   10.20.8.23    node1   
   ```

6. Select a node running connector in the cloud and label it. `node1` is used as an example:

   ```shell
   # Operate on the local member cluster.
   $ kubectl label no node1 node-role.kubernetes.io/connector=
   
   $ kubectl get node
   NAME     STATUS   ROLES       AGE     VERSION
   edge1    Ready    <none>      5h22m   v1.18.2
   edge2    Ready    <none>      5h21m   v1.18.2
   master   Ready    master      5h29m   v1.18.2
   node1    Ready    connector   5h23m   v1.18.2
   ```

   > Node：
   >
   > Select nodes that allow normal pod to run, and do not have the stain of not being able to dispatch, or the deployment will fail.  

7. Prepare the values.yaml file 

   ```shell
   # Operate on the local member cluster.
   
   $ cat > values.yaml << EOF
   operator:
     # edgePodCIDR: 10.10.0.0/16 
     connectorPublicAddresses:
     - 10.20.8.12
     serviceClusterIPRanges:
     - 10.234.0.0/18
   
     cluster:
       name: beijing
       role: member
     
     hostOperatorAPIServer: https://10.20.8.28:30303
     
     initToken: eyJ------omitted-----9u0
   
   EOF
   ```
   
   > Description：
   >
   > **edgePodCIDR**：If calico is used, it must be configured; If Flannel is used, this parameter cannot be configured.
   >
   > **connectorPublicAddresses**: The selected address of the node running the `connector` service to ensure that edge nodes can access it.  
   >
   > **serviceClusterIPRanges**: network segment used by services in cloud cluster, `service_cluster_ip_range` output by `get_cluster_info` script. 
   >
   > **cluster**: Configure the cluster name and the cluster role, the cluster name must not conflict, the first cluster must be the host role.  
   >
   > **operatorAPIServer**: Configures the `NodePor`t of the `Operator Apiserver` component. 
   
8. Install FabEdge 

   ```
   $ helm install fabedge --create-namespace -n fabedge -f values.yaml http://116.62.127.76/fabedge-0.4.0.tgz
   ```

9. Verify that services are normal on **management node**.

   ```shell
   # Operate on the local member cluster.
   
   # Verify that nodes are ready.
   $ kubectl get no
   NAME     STATUS   ROLES       AGE     VERSION
   edge1    Ready    <none>      5h22m   v1.18.2
   edge2    Ready    <none>      5h21m   v1.18.2
   master   Ready    master      5h29m   v1.18.2
   node1    Ready    connector   5h23m   v1.18.2
   
   # Verify that the Kubernetes service is normal.
   $ kubectl get po -n kube-system
   NAME                                       READY   STATUS    RESTARTS   AGE
   controlplane-master                        4/4     Running   0          159m
   coredns-546565776c-44xnj                   1/1     Running   0          159m
   coredns-546565776c-7vvnl                   1/1     Running   0          159m
   kube-flannel-ds-hbb7j                      1/1     Running   0          28m
   kube-flannel-ds-zmwbd                      1/1     Running   0          28m
   kube-proxy-47c5j                           1/1     Running   0          153m
   kube-proxy-4fckj                           1/1     Running   0          152m
   
   # Verify that the FabEdge service is normal.
   $ kubectl get po -n fabedge
   NAME                               READY   STATUS    RESTARTS   AGE
   connector-5947d5f66-hnfbv          2/2     Running   0          35m
   fabedge-agent-edge1                2/2     Running   0          22s
   fabedge-operator-dbc94c45c-r7n8g   1/1     Running   0          55s
   ```




## Create a multi-cluster Community (optional）

1. Add the cluster that needs to communicate to a Community.

   ```shell
   # Operate on the host cluster
   $ cat > community.yaml << EOF
   apiVersion: fabedge.io/v1alpha1
   kind: Community
   metadata:
     name: connectors
   spec:
     members:
       - fabedge.connector  
       - beijing.connector    
   EOF
   
   $ kubectl apply -f community.yaml
   ```
   
   > **members** is a list of endpoint names. 


## Configuration related to edge computing framework  

### If using KubeEdge

1. Modify edgecore configuration on **each edge node**.

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
       # Get clusterDNS for get_cluster_info script output.
       clusterDNS: "169.254.25.10"
       # Get clusterDomain for get_cluster_info script output.
       clusterDomain: "root-cluster"
   ```

2. Restart edgecore on **each edge node**.

   ```shell
   $ systemctl restart edgecore
   ```

### If using SuperEdge

1. Check the service status, if not Ready, remove the pod and then rebuild.

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

    SupeEdge default with a stain on the master node：node-role.kubernetes.io/master:NoSchedule,so  don't start fabedge-cloud-agent, causing pod communication on master node to fail. If necessary, the DaemonSet configuration of the fabedge-cloud-agent can be modified to tolerate this stain.  



## Configurations related to CNI

### If using Calico

No matter what clusters, as long as the cluster uses Calico, add all Pod and Service network segments of other clusters to Calico of the current cluster to prevent Calico from doing source address translation, resulting in communication failure.  

For example: host (Calico)  + member (Calico) + member(Flannel)

* Operate on the master node of the host (Calico) cluster, then configure the addresses of the other two member clusters into Calico of the host cluster.  
* Operate on the master node of the Member (Calico) cluster,then configure the addresses of the additional host (Calico) and member(Flannel) clusters to Calico of the host cluster.
* member(Flannel) No operation required. 

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

> **cidr**:  `cluster-cidr` and `service-cluster-ip-range` are output by `get_cluster_info.sh` of the added cluster.


## FaQs

1. If asymmetric routes exist on some networks, disable **rp_filter** on the cloud node  
   ```shell
   $ sudo for i in /proc/sys/net/ipv4/conf/*/rp_filter; do  echo 0 >$i; done 
   # save the configuration.
   $ sudo vi /etc/sysctl.conf
   net.ipv4.conf.default.rp_filter=0
   net.ipv4.conf.all.rp_filter=0
   ```

1. If Error is display：“Error: cannot re-use a name that is still in use”. Uninstall fabedge and try again.
   ```shell
   $ helm uninstall -n fabedge fabedge
   release "fabedge" uninstalled
   ```
