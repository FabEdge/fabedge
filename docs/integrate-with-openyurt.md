# OpenYurt和FabEdge集成

[OpenYurt](https://openyurt.io/)是托管在 Cloud Native Computing Foundation (CNCF) 下的 [沙箱项目](https://www.cncf.io/sandbox-projects/). 它是基于原生 Kubernetes 构建的，目标是扩展 Kubernetes 以无缝支持边缘计算场景。简而言之，OpenYurt 使客户可以像在公共云基础设施上运行应用一样管理在边缘基础设施之上运行的应用。

[FabEdge](https://github.com/FabEdge/fabedge)是一个专门针对边缘计算场景设计的，基于kubernetes的容器网络方案，它符合CNI规范，可以无缝集成任何K8S环境，解决边缘计算场景下云边协同，边边协同，服务发现等难题。


## 前置条件

- Kubernetes (v1.18.8)
- Flannel (v0.14.0)
- OpenYurt (v0.4.0, 至少有一个边缘节点)

## 环境准备

1. 确保所有边缘节点能够访问云端运行connector的节点

   - 如果有防火墙或安全组，必须允许ESP(50)，UDP/500，UDP/4500

1. 安装helm

     ```shell
     $ wget https://get.helm.sh/helm-v3.6.3-linux-amd64.tar.gz
     $ tar -xf helm-v3.6.3-linux-amd64.tar.gz
     $ cp linux-amd64/helm /usr/bin/helm 
     ```
1. 确保openyurt和kubernetes组件运行正常
   ```shell
   $ kubectl get po -n kube-system
   NAME                                       READY   STATUS    RESTARTS   AGE
   controlplane-master                        4/4     Running   0          121m
   coredns-546565776c-44xnj                   1/1     Running   0          121m
   coredns-546565776c-7vvnl                   1/1     Running   0          121m
   kube-flannel-ds-6wzj2                      1/1     Running   0          118m
   kube-flannel-ds-c7rdh                      1/1     Running   0          115m
   kube-flannel-ds-ggb98                      1/1     Running   0          121m
   kube-flannel-ds-sgr2k                      1/1     Running   2          114m
   kube-proxy-47c5j                           1/1     Running   0          115m
   kube-proxy-4fckj                           1/1     Running   0          114m
   kube-proxy-t48w7                           1/1     Running   0          121m
   kube-proxy-vmf4t                           1/1     Running   0          118m
   yurt-app-manager-75b7f76546-66dbv          1/1     Running   0          121m
   yurt-app-manager-75b7f76546-npx5j          1/1     Running   0          121m
   yurt-controller-manager-697877d548-5wklk   1/1     Running   0          121m
   yurt-hub-edge1                             1/1     Running   0          117m
   yurt-hub-edge2                             1/1     Running   0          115m
   yurt-tunnel-agent-kh9t5                    1/1     Running   0          117m
   yurt-tunnel-agent-rsxvs                    1/1     Running   0          114m
   yurt-tunnel-server-bc5cb5bf-5wb4l          1/1     Running   0          121m        
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
                 - key: openyurt.io/is-edge-worker
                   operator: In
                   values:
                   - "false"
                   
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
     connectorPublicAddresses: 10.20.8.28  
     connectorSubnets: 10.96.0.0/12  
     edgeLabels: openyurt.io/is-edge-worker=true
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
   > **edgeLabels**：标记边缘节点的标签。
   >
   > **cniType**: 云端集群中使用的cni插件类型。
   
3. 安装fabedge 

   ```shell
   $ helm install fabedge --create-namespace -n fabedge -f values.yaml http://116.62.127.76/fabedge-0.3.0.tgz
   ```
      > 如果出现错误：“Error: cannot re-use a name that is still in use”，是因为fabedge helm chart已经安装，使用以下命令卸载后重试。
   >```shell
   > $ helm uninstall -n fabedge fabedge
   > release "fabedge" uninstalled
   > ```

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

3. 在**管理节点**上确认OpenYurt和Kubernetes服务正常
   
   ```shell
   $ kubectl get po -n kube-system
   NAME                                       READY   STATUS    RESTARTS   AGE
   controlplane-master                        4/4     Running   0          159m
   coredns-546565776c-44xnj                   1/1     Running   0          159m
   coredns-546565776c-7vvnl                   1/1     Running   0          159m
   kube-flannel-ds-hbb7j                      1/1     Running   0          28m
   kube-flannel-ds-zmwbd                      1/1     Running   0          28m
   kube-proxy-47c5j                           1/1     Running   0          153m
   kube-proxy-4fckj                           1/1     Running   0          152m
   kube-proxy-t48w7                           1/1     Running   0          159m
   kube-proxy-vmf4t                           1/1     Running   0          155m
   yurt-app-manager-75b7f76546-66dbv          1/1     Running   0          159m
   yurt-app-manager-75b7f76546-npx5j          1/1     Running   0          159m
   yurt-controller-manager-697877d548-5wklk   1/1     Running   0          159m
   yurt-hub-edge1                             1/1     Running   0          155m
   yurt-hub-edge2                             1/1     Running   0          152m
   yurt-tunnel-agent-kh9t5                    1/1     Running   0          154m
   yurt-tunnel-agent-rsxvs                    1/1     Running   0          151m
   yurt-tunnel-server-bc5cb5bf-5wb4l          1/1     Running   0          159m
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

