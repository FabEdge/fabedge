# SuperEdge和FabEdge集成

[SuperEdge](https://github.com/superedge/superedge/blob/main/README_CN.md)是Kubernetes原生的边缘容器方案，它将Kubernetes强大的容器管理能力扩展到边缘计算场景中，针对边缘计算场景中常见的技术挑战提供了解决方案，如：单集群节点跨地域、云边网络不可靠、边缘节点位于NAT网络等。这些能力可以让应用很容易地部署到边缘计算节点上，并且可靠地运行。

[FabEdge](https://github.com/FabEdge/fabedge)是一个专门针对边缘计算场景设计的，基于kubernetes的容器网络方案，它符合CNI规范，可以无缝集成任何K8S环境，解决边缘计算场景下云边协同，边边协同，服务发现等难题。


## 前置条件

- Kubernetes (v1.18.2）
- Flannel (v0.13.0)
- SuperEdge (v0.5.0, 至少有一个边缘节点)

## 环境准备

1. 确保所有边缘节点能够访问云端运行connector的节点

   - 如果有防火墙或安全组，必须允许ESP，UDP/500，UDP/4500

1. 安装helm

     ```shell
     $ wget https://get.helm.sh/helm-v3.6.3-linux-amd64.tar.gz
     $ tar -xf helm-v3.6.3-linux-amd64.tar.gz
     $ cp linux-amd64/helm /usr/bin/helm 
     ```
1. 确保superedge和kubernetes组件正常运行
   ```shell
   $ kubectl get po -n edge-system
   NAME                                           READY   STATUS    RESTARTS   AGE
   application-grid-controller-84d64b86f9-gz9cp   1/1     Running   0          3h3m
   application-grid-wrapper-master-dx8wn          1/1     Running   0          3h3m
   edge-coredns-master3-6f8f449d4d-tkmqh          1/1     Running   0          3h2m
   edge-coredns-super1-7b454c749-bwtgx            1/1     Running   0          178m
   edge-health-admission-86c5c6dd6-9cswx          1/1     Running   0          3h3m
   edge-health-n9f7v                              1/1     Running   0          177m
   tunnel-cloud-6557fcdd67-l74x2                  1/1     Running   0          3h3m
   tunnel-coredns-7d8b48c7ff-5659c                1/1     Running   0          3h3m
   tunnel-edge-2x8mf                              1/1     Running   0          170m
   
   $ kubectl get po -n kube-system
   NAME                              READY   STATUS    RESTARTS   AGE
   coredns-5c66f5c95-6rp86           1/1     Running   0          3h3m
   etcd-master                       1/1     Running   0          3h4m
   kube-apiserver-master             1/1     Running   0          3h4m
   kube-controller-manager-master    1/1     Running   0          3h4m
   kube-flannel-ds-2b5kk             1/1     Running   0          16m
   kube-proxy-kkm6j                  1/1     Running   0          171m
   kube-scheduler-master             1/1     Running   0          3h4m
   ```
   
1. 获取当前集群配置信息，供后面使用

     ```shell
     $ curl -s http://116.62.127.76/get_cluster_info.sh | bash -
     This may take some time. Please wait.
     clusterDNS: 
     clusterDomain: kubernetes
     service-cluster-ip-range : 10.96.0.0/12
     ```


## 安装部署
1. 在master节点上，修改flannel ds，禁止其在边缘节点上运行
   ```shell
   # 创建kube-flannel-ds.patch.yaml文件
   $ vi kube-flannel-ds.patch.yaml
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
                 - key: superedge.io/edge-node
                   operator: DoesNotExist
                   
   $ kubectl patch ds -n kube-system kube-flannel-ds --patch "$(cat kube-flannel-ds.patch.yaml)"
   ```
   
1. 确认**所有边缘节点**上**没有**运行**任何**flannel的组件

   ```shell
   $ kubectl get po -n kube-system -o wide | grep -i flannel
   kube-flannel-79l8h               1/1     Running   0          3d19h   10.20.8.24    master   <none>       
   kube-flannel-8j9bp               1/1     Running   0          3d19h   10.20.8.23    node1    <none> 
   ```

1. 在云端选取一个运行connector的节点，并为它做标记，以node1为例，

   ```shell
   $ kubectl label no node1 node-role.kubernetes.io/connector=
   
   $ kubectl get node
   NAME     STATUS   ROLES       AGE     VERSION
   edge1    Ready    <none>      5h22m   v1.18.2
   edge2    Ready    <none>      5h21m   v1.18.2
   master   Ready    master      5h29m   v1.18.2
   node1    Ready    connector   5h23m   v1.18.2
   ```
   >选取的节点要允许运行普通的POD，不要有不能调度的污点，否则部署会失败。

2. 准备values.yaml文件

   ```shell
   $ vi values.yaml
   operator:
     connectorPublicAddresses: 10.20.8.23   
     connectorSubnets: 10.96.0.0/12  
     edgeLabels: superedge.io/edge-node=enable
     masqOutgoing: false
     enableProxy: false
   cniType: flannel
   ```
   
   > 说明：
   >
   > **connectorPublicAddresses**: 前面选取的，运行connector服务的节点的地址，确保能够被边缘节点访问。
   >
   > **connectorSubnets**: 云端集群中的service使用的网段，get_cluster_info脚本输出的service_cluster_ip_range。
   >
   > **edgeLabels**：标记边缘节点的标签组。
   >
   > **cniType**: 云端集群中使用的cni插件类型。
   
3. 安装fabedge 

   ```shell
   $ helm install fabedge --create-namespace -n fabedge -f values.yaml http://116.62.127.76/fabedge-0.3.0.tgz
   ```
   > 如果出现错误：“Error: cannot re-use a name that is still in use”，是因为fabedge helm chart已经安装，使用以下命令卸载后重试。
   >```shell
   > $ helm uninstall -n fabedge fabedge
   >  release "fabedge" uninstalled
   >```
   

## 部署后验证

1. 在**管理节点**上确认边缘节点就绪

    ```shell
    $ kubectl get no
    NAME     STATUS   ROLES       AGE     VERSION
    edge1    Ready    <none>      5h22m   v1.18.2
    edge2    Ready    <none>      5h21m   v1.18.2
    master   Ready    master      5h29m   v1.18.2
    node1    Ready    connector   5h23m   v1.18.2
    ```

2. 在**管理节点**上确认FabEdge服务正常

    ```shell
    $ kubectl get po -n fabedge
    NAME                               READY   STATUS    RESTARTS   AGE
    cert-zmwjg                         0/1     Completed 0          3m5s
    connector-5947d5f66-hnfbv          2/2     Running   0          35m
    fabedge-agent-edge1                2/2     Running   0          22s
    fabedge-operator-dbc94c45c-r7n8g   1/1     Running   0          55s
    ```

3. 在**管理节点**上确认superedge服务正常
   
   如果状态不正确，要删除重建，确保所有pod状态就绪。
   
   ```shell
   $ kubectl get po -n edge-system
     NAME                                           READY   STATUS    RESTARTS   AGE
      application-grid-controller-84d64b86f9-gz9cp   1/1     Running   0          3h3m
      application-grid-wrapper-master-dx8wn          1/1     Running   0          3h3m
      application-grid-wrapper-node-j4nhj            1/1     Running   0          170m
      edge-coredns-master3-6f8f449d4d-tkmqh          1/1     Running   0          3h2m
      edge-coredns-super1-7b454c749-bwtgx            0/1     Running   0          178m    # 状态错误
      edge-coredns-super2-8456587986-zhbg8           0/1     Running   0          171m    # 状态错误
      edge-health-admission-86c5c6dd6-9cswx          1/1     Running   0          3h3m
      edge-health-n9f7v                              1/1     Running   0          177m
      tunnel-cloud-6557fcdd67-l74x2                  1/1     Running   0          3h3m
      tunnel-coredns-7d8b48c7ff-5659c                1/1     Running   0          3h3m
      tunnel-edge-2x8mf                              1/1     Running   0          170m
      
   $ kubectl delete po -n edge-system edge-coredns-edge1-7b454c749-bwtgx
     pod "edge-coredns-edge1-7b454c749-bwtgx" deleted
   $ kubectl delete po -n edge-system edge-coredns-edge2-8456587986-zhbg8
     pod "edge-coredns-edge2-8456587986-zhbg8" deleted
   ```

## 常见问题
1. 有的网络环境存在非对称路由，需要在云端节点关闭rp_filter

    ```shell
    $ for i in /proc/sys/net/ipv4/conf/*/rp_filter; do  echo 0 >$i; done
    
    # 保存配置
    $ vi /etc/sysctl.conf
    net.ipv4.conf.default.rp_filter=0
    net.ipv4.conf.all.rp_filter=0
    ```
