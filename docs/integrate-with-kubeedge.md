# KubeEdge和FabEdge集成

[KubeEdge](https://github.com/kubeedge/kubeedge/blob/master/README_zh.md) 是一个开源的系统，可将本机容器化应用编排和管理扩展到边缘端设备。 它基于Kubernetes构建，为网络和应用程序提供核心基础架构支持，并在云端和边缘端部署应用，同步元数据。KubeEdge 还支持 MQTT 协议，允许开发人员编写客户逻辑，并在边缘端启用设备通信的资源约束。KubeEdge 包含云端和边缘端两部分。

[FabEdge](https://github.com/FabEdge/fabedge)是一个专门针对边缘计算场景设计的，基于kubernetes的容器网络方案，它符合CNI规范，可以无缝集成任何K8S环境，解决边缘计算场景下云边协同，边边协同，服务发现等难题。

## 前置条件

- kubernetes (v1.19.7)

- calico（v3.16.5）

- Kubeedge (v1.8.0) 

也可以[快速部署一个测试集群](https://github.com/FabEdge/fabedge/blob/main/docs/install_k8s.md)

## 环境准备

1. 确保所有边缘节点能够访问云端connector

    - 如果有防火墙或安全组，必须允许协议ESP(50)，UDP/500，UDP/4500
    
2. 确认**所有边缘节点**上[nodelocaldns](https://kubernetes.io/docs/tasks/administer-cluster/nodelocaldns/)正常运行

    ```shell
    $ kubectl get po -n kube-system -o wide -l "k8s-app=nodelocaldns"
    nodelocaldns-4m2jx                   1/1     Running     0    25m    10.22.45.30    master           
    nodelocaldns-p5h9k                   1/1     Running     0    35m    10.22.45.26    edge1      
    ```

3. 确认**所有边缘节点**上**没有**运行**任何**calico的组件
    ```shell
    $ kubectl get po -n kube-system -o wide | grep -i calico
    calico-kube-controllers-8b5ff5d58-ww4nq         1/1     Running     0          7d18h   10.22.45.30     master1   <none>           <none>
    calico-node-cxbd9                               1/1     Running     0          7d18h   10.22.45.30     master1   <none>           <none>
    ```
    
4. 安装helm

    ```shell
    $ wget https://get.helm.sh/helm-v3.6.3-linux-amd64.tar.gz
    $ tar -xf helm-v3.6.3-linux-amd64.tar.gz
    $ cp linux-amd64/helm /usr/bin/helm 
    ```
    
5. 获取当前集群配置信息，供后面使用

    ```shell
    $ curl -s http://116.62.127.76/get_cluster_info.sh | bash -
    This may take some time. Please wait.
    
    clusterDNS               : 169.254.25.10
    clusterDomain            : root-cluster
    cluster-cidr             : 10.233.64.0/18
    service-cluster-ip-range : 10.233.0.0/18
    ```

## 安装部署

### 配置calico

1. 修改calico pool配置

    ```shell
    $ vi ippool.yaml
    apiVersion: projectcalico.org/v3
    kind: IPPool
    metadata:
      name: fabedge
    spec:
      blockSize: 26
      cidr: 10.10.0.0/16
      natOutgoing: false
      disabled: true
      ipipMode: Always
    ```

    > **cidr**是一个用户自己选定的大网段，每个边缘节点会从中分配一个小网段，不能和get_cluster_info脚本输出的cluster-cidr或service-cluster-ip-range冲突。

2. 创建calico pool

    ```shell
    $ calicoctl.sh create --filename=ippool.yaml
    $ calicoctl.sh get pool  # 如果有fabedge的pool说明创建成功
    NAME           CIDR             SELECTOR   
    default-pool   10.231.64.0/18   all()      
    fabedge        10.10.0.0/16     all()
    
    # 如果提示没有calicoctl.sh命令，请尝试以下指令
    $ export DATASTORE_TYPE=kubernetes
    $ export KUBECONFIG=/etc/kubernetes/admin.conf
    $ calicoctl create --filename=ippool.yaml
    $ calicoctl get pool    # 如果有fabedge的pool说明创建成功
    NAME           CIDR             SELECTOR   
    default-pool   10.231.64.0/18   all()      
    fabedge        10.10.0.0/16     all()
    ```

### 部署FabEdge

1. 在云端选取一个运行connector的节点，并为节点做标记。以master为例，

   ```shell
   $ kubectl label no master node-role.kubernetes.io/connector=
   
   $ kubectl get node
     NAME    STATUS   ROLES                   AGE    VERSION
     edge1   Ready    agent,edge              108m   v1.19.3-kubeedge    
     master  Ready    connector,master,node   118m   v1.19.7     
   ```

2. 准备helm values.yaml文件

    ```shell
    $ vi values.yaml
    operator:
      edgePodCIDR: 10.10.0.0/16   
      connectorPublicAddresses: 10.22.46.48 
      connectorSubnets: 10.233.0.0/18
    
    cniType: calico 
    ```
    
    > 说明：
    >
    >   **edgePodCIDR**：使用上面创建的calico pool的cidr。
    >
    >   **connectorPublicAddresses**: 前面选取的，运行connector服务的节点的地址，确保能够被边缘节点访问。
    >
    >   **connectorSubnets**: 云端集群中的pod和service cidr，get_cluster_info脚本输出的service-cluster-ip-range。
    >
    >   **cniType**: 云端集群中使用的cni插件类型，当前支持calico。

1.  安装FabEdge

    ```shell
    $ helm install fabedge --create-namespace -n fabedge -f values.yaml http://116.62.127.76/fabedge-0.3.0.tgz
    ```

### 配置边缘节点

1. 在**每个边缘节点**上修改edgecore配置文件

    ```shell
    $ vi /etc/kubeedge/config/edgecore.yaml
    
    # 必须禁用edgeMesh
    edgeMesh:
      enable: false
    
    edged:
        enable: true
        cniBinDir: /opt/cni/bin
        cniCacheDirs: /var/lib/cni/cache
        cniConfDir: /etc/cni/net.d
        networkPluginName: cni
        networkPluginMTU: 1500
        # get_cluster_info脚本输出的clusterDNS
        clusterDNS: "169.254.25.10"
        # get_cluster_info脚本输出的clusterDomain
        clusterDomain: "root-cluster"
    ```

2. 在**每个边缘节点**上重启edgecore

    ```shell
    $ systemctl restart edgecore
    ```

## 部署后验证

1. 在**管理节点**上确认边缘节点就绪

    ```shell
    $ kubectl get node
      NAME    STATUS   ROLES                   AGE    VERSION
      edge1   Ready    agent,edge              125m   v1.19.3-kubeedge-v1.1.0
      master  Ready    connector,master,node   135m   v1.19.7
    ```

2. 在**管理节点**上确认FabEdge服务正常启动

    ```shell
    $ kubectl get po -n fabedge
    NAME                               READY   STATUS    RESTARTS   AGE
    cert-zmwjg                         0/1     Completed 0          3m5s
    connector-5947d5f66-hnfbv          2/2     Running   0          35m
    fabedge-agent-edge1                2/2     Running   0          22s
    fabedge-operator-dbc94c45c-r7n8g   1/1     Running   0          55s
    ```

## 常见问题

1. 有的网络环境存在非对称路由，需要在云端节点关闭rp_filter

    ```shell
    $ for i in /proc/sys/net/ipv4/conf/*/rp_filter; do  echo 0 >$i; done
    
    #保存配置
    $ vi /etc/sysctl.conf
    net.ipv4.conf.default.rp_filter=0
    net.ipv4.conf.all.rp_filter=0
    ```
    

