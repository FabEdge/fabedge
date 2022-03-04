# FabEdge安装部署


[toc]


## 概念

-  **云端集群**：标准的K8S集群，位于云端，提供云端的计算能力 
-  **边缘节点**： 使用KubeEdge等边缘计算框架，加入云端集群的边缘侧节点，提供边缘计算能力 
-  **边缘集群**：标准的K8S集群，位于边缘侧，提供边缘计算能力 
-  **Host集群**：选定的一个云端集群，用于管理其它集群的跨集群通讯，FabEdge部署的第一个集群必须是host集群 
-  **member集群**：一个边缘集群，注册到host集群，上报本集群端点网络配置信息，用于多集群通讯 
-  **Community**：FabEdge定义的CRD，有两种场景： 
   - 定义一个集群内多个边缘节点之间的通讯
   - 定义多个边缘集群之间的通讯



## 前置条件

- Kubernetes (v1.18.8)
- Flannel (v0.14.0) 或者 Calico (v3.16.5)



## 环境准备

1.  确保防火墙或安全组允许以下协议和端口 
   - ESP(50)，UDP/500，UDP/4500



## 在主集群部署FabEdge

1.  安装FabEdge   
```shell
$ curl 116.62.127.76/installer/latest/install.sh | bash -s -- --cluster-name beijing  --cluster-role host --cluster-zone beijing  --cluster-region haidian --edges edge1 --connectors node1 --connector-public-addresses 10.22.46.47 --chart http://116.62.127.76/fabedge-0.5.0.tgz
```
> 说明：
>  
> **--cluster-name**: 集群名称
>  
> **--cluster-role**: 集群角色
>  
> **--edges**: 边缘节点主机名列表，用逗号分隔
>  
> **--connectors**: connectors所在节点主机名
>  
> **--connector-public-addresses**: connectors所在节点的ip地址

2.  确认**所有边缘节点**上**没有**运行**任何**CNI的组件  
```shell
$ kubectl get po -n kube-system -o wide | egrep -i "flannel|calico"
calico-kube-controllers-8b5ff5d58-d2pkj   1/1     Running   0          67m   10.20.8.20    master
calico-node-t5vww                         1/1     Running   0          38s   10.20.8.28    node1
calico-node-z2fmf                         1/1     Running   0          62s   10.20.8.20    master
```

3.  确认服务正常  
```shell
# 在master上执行
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

4.  为需要通讯的边缘节点创建节点Community  
```shell
# 在master节点执行
$ cat > node-community.yaml << EOF
apiVersion: fabedge.io/v1alpha1
kind: Community
metadata:
  name: connectors
spec:
  members:
    - beijing.edge1
    - beijing.edge2  
EOF

$ kubectl apply -f node-community.yaml
```

5.  根据使用的[边缘计算框架](#%E5%92%8C%E8%BE%B9%E7%BC%98%E8%AE%A1%E7%AE%97%E6%A1%86%E6%9E%B6%E7%9B%B8%E5%85%B3%E7%9A%84%E9%85%8D%E7%BD%AE)修改相关配置 
5.  根据使用的[CNI](#%E5%92%8CCNI%E7%9B%B8%E5%85%B3%E7%9A%84%E9%85%8D%E7%BD%AE)修改相关配置 



## 在成员集群部署FabEdge
如果有多集群，须要在每个成员集群部署FabEdge，否则跳过本步

1.  在**host集群**，添加一个名字叫“shai”的成员集群，获取Token供注册使用  
```shell
# 在host集群master节点上执行
$ cat > shai.yaml << EOF
apiVersion: fabedge.io/v1alpha1
kind: Cluster
metadata:
  name: shai # 集群名字
EOF

$ kubectl apply -f shai.yaml

$ kubectl get cluster shai -o go-template --template='{{.spec.token}}' | awk 'END{print}' 
eyJ------省略内容-----9u0
```

2.  在**成员集群**安装FabEdage   
```shell
curl 116.62.127.76/installer/latest/install.sh | bash -s -- --cluster-name shai --cluster-role member --cluster-zone shai  --cluster-region pudong --edges edge1 --connectors node1 --chart http://116.62.127.76/fabedge-0.5.0.tgz --server-serviceHub-api-server https://10.22.46.47:30304 --host-operator-api-server https://10.22.46.47:30303 --connector-public-addresses 10.22.46.26 --init-token ey...Jh
```
> 说明：
>  
> **--cluster-name**: 集群名称
>  
> **--cluster-role**: 集群角色
>  
> **--edges**: 边缘节点主机名，多个的话edge1,edge2用逗号进行分割
>  
> **--server-serviceHub-api-server**: host集群serviceHub服务的地址和端口
>  
> **--host-operator-api-server**: host集群operator-api服务的地址和端口
>  
> **--connector-public-addresses**: member集群connectors所在节点的ip地址
>  
> **--init-token**: host集群获取的token



## 启用多集群通讯
如果使用多集群通讯，须要创建一个集群类型的Community，否则跳过本步。

1.  在host集群，把需要通讯的集群加入一个Community  
```shell
# 在master节点操作
$ cat > community.yaml << EOF
apiVersion: fabedge.io/v1alpha1
kind: Community
metadata:
  name: connectors
spec:
  members:
    - shai.connector   # {集群名字}.connector
    - beijing.connector   # {集群名字}.connector
EOF

$ kubectl apply -f community.yaml
```


## 启用多集群服务发现
如果使用多集群通讯，须要修改集群DNS配置，否则跳过本步。
须要修改的组件：

- 如果使用了nodelocaldns，只须要修改nodelocaldns，其它不用配置。
- 如果使用SuperEdge edge-coredns的，修改coredns + edge-coredns，其它不用配置。
- 其它情况只需要修改coredns。



1.  配置nodelocaldns  
```shell
$ kubectl -n kube-system edit cm nodelocaldns
global:53 {
        errors
        cache 30
        reload
        bind 169.254.25.10                 # 本地bind地址，参考其它配置段中的bind
        forward . 10.233.12.205            # fandns的service IP地址
    }
```

2.  配置edge-coredns  
```shell
$ kubectl -n edge-system edit cm edge-coredns
global {
   forward . 10.244.51.126                 # fandns的service IP地址
}
```

3.  配置coredns  
```shell
$ kubectl -n kube-system edit cm coredns
global {
   forward . 10.109.72.43                 # fandns的service IP地址
}
```
> 修改configmap后，须要重启coredns、edge-coredns和nodelocaldns保证变更生效



## 与边缘计算框架相关的配置
获取当前集群配置信息，供后面使用
```shell
$ curl -s http://116.62.127.76/installer/latest/get_cluster_info.sh | bash -
This may take some time. Please wait.

clusterDNS               : 169.254.25.10
clusterDomain            : root-cluster
cluster-cidr             : 10.233.64.0/18
service-cluster-ip-range : 10.233.0.0/18
```


### 如果使用KubeEdge

1.  确认nodelocaldns在**边缘节点**正常运行  
```shell
$ kubectl get po -n kube-system -o wide | grep nodelocaldns
nodelocaldns-cz5h2                        1/1     Running   0          56m   10.22.46.47   master   <none>           <none>
nodelocaldns-nk26g                        1/1     Running   0          47m   10.22.46.23   edge1    <none>           <none>
nodelocaldns-wqpbw                        1/1     Running   0          17m   10.22.46.20   node1    <none>           <none>
```

2.  在**每个边缘节点**上修改edgecore配置   
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

3.  在**每个边缘节点**上重启edgecore  
```shell
$ systemctl restart edgecore
```


### 如果使用SuperEdge

1.  检查服务状态，如果不Ready，要删除Pod重建  
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

2.  master节点上的Pod不能和边缘Pod通讯
SupeEdge的master节点上默认带有污点：node-role.kubernetes.io/master:NoSchedule， 所以不会启动fabedge-cloud-agent， 导致不能和master节点上的Pod通讯。如果需要，可以修改fabedge-cloud-agent的DaemonSet配置，容忍这个污点。 



## 与CNI相关的配置
获取当前集群配置信息，供后面使用


```shell
$ curl -s http://116.62.127.76/installer/latest/get_cluster_info.sh | bash -
This may take some time. Please wait.

clusterDNS               : 169.254.25.10
clusterDomain            : root-cluster
cluster-cidr             : 10.233.64.0/18
service-cluster-ip-range : 10.233.0.0/18
```


### 如果使用Calico
不论是什么集群角色, 只要集群使用Calico，就要将其它所有集群的Pod和Service的网段加入当前集群的Calico配置,  防止Calico做源地址转换，导致不能通讯。
例如: host (Calico)  + member1 (Calico) + member2 (Flannel)

- 在host (Calico) 集群的master节点操作，将member1 (Calico)，member2 (Flannel)地址配置到host集群的Calico中。
- 在member1 (Calico)集群的master节点操作，将host (Calico) ，member2 (Flannel)地址配置到member1集群的Calico中。
- 在member2 (Flannel)无需任何操作。
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

## 
## 常见问题

1.  有的网络环境存在非对称路由，需要在云端节点关闭rp_filter  
```shell
$ sudo for i in /proc/sys/net/ipv4/conf/*/rp_filter; do  echo 0 >$i; done 
# 保存配置
$ sudo vi /etc/sysctl.conf
net.ipv4.conf.default.rp_filter=0
net.ipv4.conf.all.rp_filter=0
```

2.  报错：“Error: cannot re-use a name that is still in use”。这是因为fabedge已经安装，使用以下命令卸载后重试。  
```shell
$ helm uninstall -n fabedge fabedge
release "fabedge" uninstalled
```
