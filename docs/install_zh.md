# FabEdge安装部署

[toc]

## 概念

- **云端集群**：一个标准的K8S集群，位于云端，提供云端的计算能力
- **边缘节点**： 使用KubeEdge等边缘计算框架，将一个节点加入云端集群，提供边缘计算能力
- **边缘集群**：一个标准的K8S集群，位于边缘侧，提供边缘计算能力

- **host集群**：选定的一个云端集群，用于管理其它集群的跨集群通讯（FabEdge部署的第一个集群必须是host集群）
- **member集群**：一个边缘集群，注册到host集群，上报本集群端点网络配置信息，用于多集群通讯
- **Community**：FabEdge定义的CRD，有两种场景：
  - 定义一个集群内多个边缘节点之间的通讯
  - 定义多个边缘集群之间的通讯



## 前置条件

- Kubernetes (v1.18.8)
- Flannel (v0.14.0) 或者 Calico (v3.16.5)



## 环境准备

1. 确保防火墙或安全组允许以下协议和端口

   - ESP(50)，UDP/500，UDP/4500

1. 为每个集群安装helm

     ```shell
     $ wget https://get.helm.sh/helm-v3.6.3-linux-amd64.tar.gz
     $ tar -xf helm-v3.6.3-linux-amd64.tar.gz
     $ cp linux-amd64/helm /usr/bin/helm 
     ```
     



## 在host集群里部署FabEdge

1. 获取当前集群配置信息，供后面使用

   ```shell
   $ curl -s http://116.62.127.76/get_cluster_info.sh | bash -
   This may take some time. Please wait.
   
   clusterDNS               : 169.254.25.10
   clusterDomain            : root-cluster
   cluster-cidr             : 10.233.64.0/18
   service-cluster-ip-range : 10.233.0.0/18
   ```

2. 为**所有边缘节点**添加标签

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
   
3. 修改CNI的配置，禁止其在边缘节点上运行

   ```bash
   # 在master节点上执行
   
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
   
   # 如果是Flannel
   $ kubectl patch ds -n kube-system kube-flannel-ds --patch "$(cat cni-ds.patch.yaml)"
   
   # 如果是Calico
   $ kubectl patch ds -n kube-system calico-node --patch "$(cat cni-ds.patch.yaml)"
   ```

4. 确认**所有边缘节点**上**没有**运行**任何**CNI的组件

   ```shell
   $ kubectl get po -n kube-system -o wide | egrep -i "flannel|calico"
   calico-kube-controllers-8b5ff5d58-d2pkj   1/1     Running   0          67m   10.20.8.20    master
   calico-node-t5vww                         1/1     Running   0          38s   10.20.8.28    node1
   calico-node-z2fmf                         1/1     Running   0          62s   10.20.8.20    master
   ```

5. 在云端选取一个节点运行connector，为它做标记，以node1为例

   ```shell
   $ kubectl label no node1 node-role.kubernetes.io/connector=
   $ kubectl get node
   NAME     STATUS   ROLES       AGE     VERSION
   edge1    Ready    edge        5h22m   v1.18.2
   edge2    Ready    edge        5h21m   v1.18.2
   master   Ready    master      5h29m   v1.18.2
   node1    Ready    connector   5h23m   v1.18.2
   ```

   > 注意：选取的节点要允许运行普通的Pod，不要有不能调度的污点，否则部署会失败

6. 准备values.yaml文件

   ```shell
   # 在master上执行
   $ cat > values.yaml << EOF
   operator:
     edgePodCIDR: 10.10.0.0/16   # 如果使用calico，必须配置；如果使用Flannel，不能配置
     connectorPublicAddresses:
     - 10.20.8.28
     serviceClusterIPRanges:
     - 10.233.0.0/18
       
     cluster:
       name: fabedge  # 集群的名字
       role: host     # 集群角色，第一个集群必须是host
     
     operatorAPIServer:
       nodePort: 30303
   
   EOF
   ```
   
   > 说明：
   >
   > **connectorPublicAddresses**: 前面选取的，运行connector服务的节点的地址，确保能够被边缘节点访问
   >
   > **serviceClusterIPRanges**: 云端集群中的service使用的网段，get_cluster_info脚本输出的service_cluster_ip_range
   >
   > **cluster**: 配置集群名称和集群角色，集群名字不能冲突， 第一个集群必须是host角色
   >
   > **operatorAPIServer**: Operator API server使用的NodePort
   
7. 安装FabEdge 

   ```
   $ helm install fabedge --create-namespace -n fabedge -f values.yaml http://116.62.127.76/fabedge-0.4.0.tgz
   ```

8. 确认服务正常

   ```shell
   # 在master上执行
   $ kubectl get no
   NAME     STATUS   ROLES       AGE     VERSION
   edge1    Ready    edge        5h22m   v1.18.2
   edge2    Ready    edge        5h21m   v1.18.2
   master   Ready    master      5h29m   v1.18.2
   node1    Ready    connector   5h23m   v1.18.2
   
   $ kubectl get po -n kube-system
   NAME                                       READY   STATUS    RESTARTS   AGE
   controlplane-master                        4/4     Running   0          159m
   coredns-546565776c-44xnj                   1/1     Running   0          159m
   coredns-546565776c-7vvnl                   1/1     Running   0          159m
   kube-flannel-ds-hbb7j                      1/1     Running   0          28m
   kube-flannel-ds-zmwbd                      1/1     Running   0          28m
   kube-proxy-47c5j                           1/1     Running   0          153m
   kube-proxy-4fckj                           1/1     Running   0          152m
   
   $ kubectl get po -n fabedge
   NAME                               READY   STATUS    RESTARTS   AGE
   connector-5947d5f66-hnfbv          2/2     Running   0          35m
   fabedge-agent-edge1                2/2     Running   0          22s
   fabedge-operator-dbc94c45c-r7n8g   1/1     Running   0          55s
   
   ```
   
9. 为需要通讯的边缘节点创建Community

   ```shell
   # 在master节点执行
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
   



## 在member集群里部署FabEdge（可选）

1. 在**host集群**，添加一个名字叫“beijing”的成员集群，获取Token供注册使用

   ```shell
   # 在master节点上执行
   $ cat > beijing.yaml << EOF
   apiVersion: fabedge.io/v1alpha1
   kind: Cluster
   metadata:
     name: beijing # 集群名字
   EOF
   
   $ kubectl apply -f beijing.yaml
   
   $ kubectl get cluster beijing -o go-template --template='{{.spec.token}}' | awk 'END{print}' 
   eyJ------省略内容-----9u0 
   ```

7. 在**成员集群**，准备values.yaml文件

   ```shell
   # 在master上执行
   $ cat > values.yaml << EOF
   operator:
     edgePodCIDR: 10.10.0.0/16  # 如果使用calico，必须配置；如果使用Flannel，不能配置
     connectorPublicAddresses:
     - 10.20.8.12
     serviceClusterIPRanges:
     - 10.234.0.0/18
   
     cluster:
       name: beijing # 集群名字
       role: member  # 必须是“member”
     
     hostOperatorAPIServer: https://10.20.8.28:30303  # host集群里operator api server的地址
     
     initToken: eyJ------省略内容-----9u0   # 在host集群添加成员集群时生成的token
   
   EOF
   ```
   
8. 其它步骤和host集群的部署相同，这里不再重复，请参考前面章节。

   
   


## 创建集群Community（可选）

1. 在host集群，把需要通讯的集群加入一个Community

   ```shell
   # 在master节点操作
   $ cat > community.yaml << EOF
   apiVersion: fabedge.io/v1alpha1
   kind: Community
   metadata:
     name: connectors
   spec:
     members:
       - fabedge.connector   # {集群名字}.connector
       - beijing.connector   # {集群名字}.connector
   EOF
   
   $ kubectl apply -f community.yaml
   ```
   



## 和边缘计算框架相关的配置

### 如果使用KubeEdge

1. 确认nodelocaldns在**边缘节点**正常运行

   ```shell
   $ kubectl get po -n kube-system -o wide | grep nodelocaldns
   nodelocaldns-ckpb4                              1/1     Running        1          6d5h    10.22.46.15     node1    <none>           <none>
   nodelocaldns-drmlz                              0/1     Running        0          2m50s   10.22.46.40     edge1   <none>           <none>
   nodelocaldns-vbxf9                              1/1     Running        1          4h6m    10.22.46.23     master   <none>           <none>
   ```
   
2. 在**每个边缘节点**上修改edgecore配置

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
       clusterDNS: 169.254.25.10        # get_cluster_info脚本输出的clusterDNS
       clusterDomain: "root-cluster"    # get_cluster_info脚本输出的clusterDomain
   ```

   > **clusterDNS**：如果没有启用nodelocaldns，请使用coredns service的地址

3. 在**每个边缘节点**上重启edgecore

   ```shell
   $ systemctl restart edgecore
   ```

### 如果使用SuperEdge

1. 检查服务状态，如果不Ready，要删除Pod重建

    ```shell
    # 在master节点执行
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

1. master节点上的Pod不能和边缘Pod通讯

    SupeEdge的master节点上默认带有污点：node-role.kubernetes.io/master:NoSchedule， 所以不会启动fabedge-cloud-agent， 导致不能和master节点上的Pod通讯。如果需要，可以修改fabedge-cloud-agent的DaemonSet配置，容忍这个污点。



## 和CNI相关的配置

### 如果使用Calico

不论是什么集群角色, 只要集群使用Calico，就要将其它所有集群的Pod和Service的网段加入当前集群的Calico配置,  防止Calico做源地址转换，导致不能通讯。

例如: host (Calico)  + member1 (Calico) + member2 (Flannel)

* 在host (Calico) 集群的master节点操作，将member1 (Calico)，member2 (Flannel)地址配置到host集群的Calico中。
* 在member1 (Calico)集群的master节点操作，将host (Calico) ，member2 (Flannel)地址配置到member1集群的Calico中。
* 在member2 (Flannel)无需任何操作。

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

> **cidr**: 被添加集群的get_cluster_info.sh输出的cluster-cidr和service-cluster-ip-range



## 常见问题

1. 有的网络环境存在非对称路由，需要在云端节点关闭rp_filter
   ```shell
   $ sudo for i in /proc/sys/net/ipv4/conf/*/rp_filter; do  echo 0 >$i; done 
   # 保存配置
   $ sudo vi /etc/sysctl.conf
   net.ipv4.conf.default.rp_filter=0
   net.ipv4.conf.all.rp_filter=0
   ```

1. 报错：“Error: cannot re-use a name that is still in use”。这是因为fabedge已经安装，使用以下命令卸载后重试。
   ```shell
   $ helm uninstall -n fabedge fabedge
   release "fabedge" uninstalled
   ```
