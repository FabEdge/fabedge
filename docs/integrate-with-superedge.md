# SuperEdge和FabEdge集成

[SuperEdge](https://github.com/superedge/superedge/blob/main/README_CN.md)是Kubernetes原生的边缘容器方案，它将Kubernetes强大的容器管理能力扩展到边缘计算场景中，针对边缘计算场景中常见的技术挑战提供了解决方案，如：单集群节点跨地域、云边网络不可靠、边缘节点位于NAT网络等。这些能力可以让应用很容易地部署到边缘计算节点上，并且可靠地运行。

[FabEdge](https://github.com/FabEdge/fabedge)是一个专门针对边缘计算场景设计的，基于kubernetes的容器网络方案，它符合CNI规范，可以无缝集成任何K8S环境，解决边缘计算场景下云边协同，边边协同，服务发现等难题。


## 前置条件

- Kubernetes (v1.18.2）
- Flannel (v0.13.0)
- SuperEdge (v0.5.0, 至少有一个边缘节点)
- 操作系统
  - Ubuntu 18.04.5 Server 4.15.0-136-generic (**推荐使用**）
  - CentOS Linux release 8.0.1905 (Core)

## 环境准备

1. 确保所有边缘节点能够访问云端运行connector的节点

   - 如果有防火墙或安全组，必须允许ESP(50)，UDP/500，UDP/4500

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
   application-grid-controller-84d64b86f9-2zpvg   1/1     Running   0          12m
   application-grid-wrapper-master-x75nh          1/1     Running   0          12m
   application-grid-wrapper-node-hdpt4            1/1     Running   0          5m7s
   application-grid-wrapper-node-v4xpq            1/1     Running   0          7m19s
   edge-coredns-edge1-5758f9df57-czp8h            1/1     Running   0          7m47s
   edge-coredns-edge2-84fd9cfd98-dhdjr            1/1     Running   0          5m45s
   edge-coredns-master-f8bf9975c-9jf4b            1/1     Running   0          11m
   edge-health-9qkjd                              1/1     Running   0          5m7s
   edge-health-admission-86c5c6dd6-zdbt9          1/1     Running   0          12m
   edge-health-ptwl5                              1/1     Running   1          7m19s
   tunnel-cloud-6557fcdd67-dkdpz                  1/1     Running   0          12m
   tunnel-coredns-7d8b48c7ff-k86sq                1/1     Running   0          12m
   tunnel-edge-jf72f                              1/1     Running   0          5m7s
   tunnel-edge-snqtw                              1/1     Running   0          7m19s
   
   $ kubectl get po -n kube-system
   NAME                             READY   STATUS    RESTARTS   AGE
   coredns-5c66f5c95-7rkgq          1/1     Running   0          12m
   coredns-5c66f5c95-cnlgz          1/1     Running   0          12m
   etcd-master                      1/1     Running   0          13m
   kube-apiserver-master            1/1     Running   0          13m
   kube-controller-manager-master   1/1     Running   0          13m
   kube-flannel-ds-6pw82            1/1     Running   3          6m
   kube-flannel-ds-6qf8j            1/1     Running   3          8m3s
   kube-flannel-ds-hw42q            1/1     Running   0          12m
   kube-flannel-ds-rn8n7            1/1     Running   4          10m
   kube-proxy-kpffx                 1/1     Running   0          10m
   kube-proxy-ktddk                 1/1     Running   0          6m
   kube-proxy-mvfrh                 1/1     Running   0          8m2s
   kube-proxy-zpszh                 1/1     Running   0          11m
   kube-scheduler-master            1/1     Running   0          13m
   ```
   
1. 获取当前集群配置信息，供后面使用

     ```shell
     $ curl -s http://116.62.127.76/get_cluster_info.sh | bash -
     This may take some time. Please wait.
     
     clusterDNS               : 
     clusterDomain            : kubernetes
     cluster-cidr             : 192.168.0.0/16
     service-cluster-ip-range : 10.96.0.0/12
     ```


## 安装部署
1. 为**所有边缘节点**添加标签
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
   
2. 在master节点上，修改flannel配置，禁止其在边缘节点上运行，创建kube-flannel-ds.patch.yaml文件

   ```shell
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
                 - key: node-role.kubernetes.io/edge   # label to identify all edge nodes added before
                   operator: DoesNotExist
                   
   $ kubectl patch ds -n kube-system kube-flannel-ds --patch "$(cat kube-flannel-ds.patch.yaml)"
   ```
   > 使用前面为边缘节点添加的标签的Key

3. 确认**所有边缘节点**上**没有**运行**任何**flannel的组件

   ```shell
   $ kubectl get no; kubectl get po -n kube-system -o wide | grep -i flannel
   NAME     STATUS   ROLES    AGE   VERSION
   edge1    Ready    edge     22h   v1.18.2
   edge2    Ready    edge     22h   v1.18.2
   master   Ready    master   22h   v1.18.2
   node1    Ready    <none>   22h   v1.18.2
   
   kube-flannel-ds-56x59            1/1     Running   0          47s   10.20.8.23    node1    <none>         
   kube-flannel-ds-92sw6            1/1     Running   0          7s    10.20.8.24    master   <none>         
   ```

4. 在云端选取一个运行connector的节点，并为它做标记，以node1为例，

   ```shell
   $ kubectl label no node1 node-role.kubernetes.io/connector=; kubectl get node
     node/node1 labeled
     NAME     STATUS   ROLES       AGE   VERSION
     edge1    Ready    edge        22h   v1.18.2
     edge2    Ready    edge        22h   v1.18.2
     master   Ready    master      22h   v1.18.2
     node1    Ready    connector   22h   v1.18.2
   ```
   >SuperEdge默认的master节点上有不能调度的污点，不能选取它运行connector。

5. 准备values.yaml文件

   ```shell
   $ vi values.yaml
   operator:
     connectorPublicAddresses: 10.20.8.23   
     connectorSubnets: 10.96.0.0/12  
     edgeLabels: node-role.kubernetes.io/edge
     masqOutgoing: true
     enableProxy: false
   cniType: flannel
   ```

   > 说明：
   >
   > **connectorPublicAddresses**: 前面选取的，运行connector服务的节点的地址，确保能够被边缘节点访问。
   >
   > **connectorSubnets**: 云端集群中的service使用的网段，get_cluster_info脚本输出的service_cluster_ip_range。
   >
   > **edgeLabels**：使用前面为所有边缘节点添加的标签。
   >
   > **cniType**: 云端集群中使用的cni插件类型。

6. 安装fabedge 

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
    NAME     STATUS   ROLES       AGE    VERSION
    edge1    Ready    edge        97m    v1.18.2
    edge2    Ready    edge        95m    v1.18.2
    master   Ready    master      102m   v1.18.2
    node1    Ready    connector   99m    v1.18.2
    ```

2. 在**管理节点**上确认FabEdge服务正常

    ```shell
    $ kubectl get po -n fabedge
    NAME                                READY   STATUS      RESTARTS   AGE
    cert-vcbb9                          0/1     Completed   0          87s
    connector-5767476fd4-82bfg          2/2     Running     0          69s
    fabedge-agent-edge1                 2/2     Running     0          57s
    fabedge-agent-edge2                 2/2     Running     0          56s
    fabedge-operator-6bdd896c45-45nlc   1/1     Running     0          69s
    ```

3. 在**管理节点**上确认superedge服务正常
   
   如果状态不正确，要删除重建，确保所有pod状态就绪。
   
   ```shell
   $ kubectl get po -n edge-system
   NAME                                           READY   STATUS    RESTARTS   AGE
   application-grid-controller-84d64b86f9-2zpvg   1/1     Running   0          103m
   application-grid-wrapper-master-x75nh          1/1     Running   0          103m
   application-grid-wrapper-node-hdpt4            1/1     Running   0          96m
   application-grid-wrapper-node-v4xpq            1/1     Running   0          98m
   edge-coredns-edge1-5758f9df57-czp8h            0/1     Running   1          99m    # 状态错误
   edge-coredns-edge2-84fd9cfd98-dhdjr            0/1     Running   1          97m    # 状态错误
   edge-coredns-master-f8bf9975c-9jf4b            1/1     Running   0          102m
   edge-health-9qkjd                              1/1     Running   0          96m
   edge-health-admission-86c5c6dd6-zdbt9          1/1     Running   0          103m
   edge-health-ptwl5                              1/1     Running   1          98m
   tunnel-cloud-6557fcdd67-dkdpz                  1/1     Running   0          103m
   tunnel-coredns-7d8b48c7ff-k86sq                1/1     Running   0          103m
   tunnel-edge-jf72f                              1/1     Running   0          96m
   tunnel-edge-snqtw                              1/1     Running   0          98m
      
   $ kubectl delete po -n edge-system edge-coredns-edge1-5758f9df57-czp8h
     pod "edge-coredns-edge1-5758f9df57-czp8h" deleted
   $ kubectl delete po -n edge-system edge-coredns-edge2-84fd9cfd98-dhdjr
     pod "edge-coredns-edge2-84fd9cfd98-dhdjr" deleted
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
