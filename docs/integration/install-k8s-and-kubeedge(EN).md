# Deploy the K8S cluster

## Installation condition

- follow [kubeadm minimum requirements](https://kubernetes.io/zh/docs/setup/production-environment/tools/kubeadm/install-kubeadm/#before-you-begin) ，Master && Node minimum 2C2G, and the disk space is not less than10G.

  > ⚠️Note：Use as clean a system as possible to avoid installation errors caused by other factors.  

## Supported operating systems

- **Ubuntu 18.04 （recommend）**
- Ubuntu 20.04
- CentOS 7.9 
- CentOS 7.8

## Deploy the K8S cluster  

### Install the K8S Master node

Using Ubuntu 18.04.5 as an example, run the following commands:  

```shell
sudo curl http://116.62.127.76/FabEdge/fabedge/main/deploy/cluster/install-k8s.sh | bash -
```

> ⚠️Note: If the loading time is too long, the network speed may be slow. Please wait patiently.

If the following information is displayed, the installation is successful:

```
PLAY RECAP *********************************************************************
master                     : ok=15   changed=13   unreachable=0    failed=0    skipped=0    rescued=0    ignored=0
```

### Add a K8S edge node

```shell
sudo curl http://116.62.127.76/FabEdge/fabedge/main/deploy/cluster/add-edge-node.sh | bash -s -- --host-vars ansible_hostname={hostname} ansible_user={username} ansible_password={password} ansible_host={edge-node-IP}
```

Description：

* ansible_hostname	 Specifies the hostname of the edge node

* ansible_user               Specifies the username of the edge node

* ansible_password      Specifies the password of the edge node

* ansible_host               Specifies the ip address of the edge node

  For example：set the host name edge1, user name root, password pwd111, and IP address 10.22.45.26 of the edge node as follows:  

  ```shell
  sudo curl http://116.62.127.76/FabEdge/fabedge/main/deploy/cluster/add-edge-node.sh |  bash -s -- --host-vars ansible_hostname=edge1 ansible_user=root ansible_password=pwd111 ansible_host=10.22.45.26
  ```

If the following information is displayed, the installation is successful:

```
PLAY RECAP *********************************************************************
edge1                      : ok=13   changed=10   unreachable=0    failed=0    skipped=1    rescued=0    ignored=0 
```

### Verify that the node is added successfully

```shell
sudo kubectl get node
NAME     STATUS   ROLES                   AGE     VERSION
edge1    Ready    agent,edge              22m      v1.19.3-kubeedge-v1.5.0
master   Ready    master,node             32m      v1.19.7    
```

> ⚠️Note: If the edge node is not configured with a password, you need to configure an SSH certificate. 
>
> Configure the SSH certificate for the master node:
>
> ```shell
> sudo docker exec -it installer bash
> sudo ssh-copy-id {edge-node-IP}
> ```

