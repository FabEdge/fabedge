# 部署k8s集群

## 安装条件

- 遵循 [kubeadm的最低要求](https://kubernetes.io/zh/docs/setup/production-environment/tools/kubeadm/install-kubeadm/#before-you-begin) ，Master && Node 最低2C2G，磁盘空间不小于10G；

  > ⚠️注意：尽可能使用干净的系统，避免其他因素引起安装错误。

## 支持的操作系统

- **Ubuntu 18.04 （推荐使用）**
- Ubuntu 20.04
- CentOS 7.9 
- CentOS 7.8

## 部署k8s集群

### 安装k8s Master 节点

以Ubuntu 18.04.5 系统为例子，运行以下指令：

```shell
sudo curl http://116.62.127.76/FabEdge/fabedge/main/deploy/cluster/install-k8s.sh | bash -
```

> ⚠️注意：如果加载时间过长，有可能网速较慢，请耐心等待

如果出现以下信息，表示安装成功：

```
PLAY RECAP *********************************************************************
master                     : ok=15   changed=13   unreachable=0    failed=0    skipped=0    rescued=0    ignored=0
```

### 添加k8s边缘节点

```shell
sudo curl http://116.62.127.76/FabEdge/fabedge/main/deploy/cluster/add-edge-node.sh | bash -s -- --host-vars ansible_hostname={hostname} ansible_user={username} ansible_password={password} ansible_host={edge-node-IP}
```

参数说明：

* ansible_hostname	 指定边缘节点的主机名

* ansible_user               配置边缘节点的用户名

* ansible_password      配置边缘节点的密码

* ansible_host               配置边缘节点的IP地址

  例如：设置边缘节点的主机名为edge1、用户名是root、密码是pwd111、IP为10.22.45.26，指令如下：

  ```shell
  sudo curl http://116.62.127.76/FabEdge/fabedge/main/deploy/cluster/add-edge-node.sh |  bash -s -- --host-vars ansible_hostname=edge1 ansible_user=root ansible_password=pwd111 ansible_host=10.22.45.26
  ```

如果出现以下信息，表示安装成功：

```
PLAY RECAP *********************************************************************
edge1                      : ok=13   changed=10   unreachable=0    failed=0    skipped=1    rescued=0    ignored=0 
```

### 确认节点添加成功

```shell
sudo kubectl get node
NAME     STATUS   ROLES                   AGE     VERSION
edge1    Ready    agent,edge              22m      v1.19.3-kubeedge-v1.5.0
master   Ready    master,node             32m      v1.19.7    
```

> ⚠️注意：如果边缘节点没有配置密码，需要配置ssh证书。
>
> master节点配置ssh证书：
>
> ```shell
> sudo docker exec -it installer bash
> sudo ssh-copy-id {edge-node-IP}
> ```

