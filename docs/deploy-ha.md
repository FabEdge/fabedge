# Deploy HA FabEdge

FabEdge has implemented HA in v1.0.0 and this article will show you how to deploy HA FabEdge.  

*PS: About how to configure edge frameworks and DNS, please checkout [Get Started](./get-started.md), We won't repeat it again.*       

## Enviroment

- Kubernetes v1.22.5

- Flannel v0.19.2

- KubeEdge 1.12.2

- Helm3

  

Nodes:

```shell
NAME    STATUS   ROLES                  AGE     VERSION                    INTERNAL-IP    EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION      CONTAINER-RUNTIME
edge1   Ready    agent,edge             6d21h   v1.22.6-kubeedge-v1.12.2   10.22.53.116   <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://20.10.21
edge4   Ready    agent,edge             6d22h   v1.22.6-kubeedge-v1.12.2   10.40.30.110   <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://24.0.5
harry   Ready    control-plane,master   20d     v1.22.5                    192.168.1.5    <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://24.0.5
node1   Ready    <none>                 20d     v1.22.6                    192.168.1.6    <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://24.0.5
node2   Ready    <none>                 135m    v1.22.5                    192.168.1.7    <none>        Ubuntu 20.04.6 LTS   5.4.0-166-generic   docker://24.0.5
```

harry, node1, node2 are cloud nodes and connect to a gateway whose public address is 10.40.10.180, edge1 and edge4 are located in their own networks.

## Deploy

1. Add helm chart repo: 

```shell
helm repo add fabedge https://fabedge.github.io/helm-chart
```

2. Get network information from cluster:

```
curl -s https://fabedge.github.io/helm-chart/scripts/get_cluster_info.sh | bash -
This may take some time. Please wait.

clusterDNS               : 
clusterDomain            : kubernetes
cluster-cidr             : 10.233.64.0/18
service-cluster-ip-range : 10.233.0.0/18
```

3. Execute quickstart.sh

```shell
curl https://fabedge.github.io/helm-chart/scripts/quickstart.sh | bash -s -- \
    --cluster-name harry \
    --cluster-region harry \
    --cluster-zone harry \
    --cluster-role host \
    --connectors node1,node2 \
    --edges edge1,edge4 \
    --connector-public-addresses 10.40.10.180 \
    --connector-public-port 45000 \
    --connector-as-mediator true \
    --enable-keepalived true \
    --keepalived-vip 192.168.1.200 \
    --keepalived-interface enp0s3 \
    --keepalived-router-id 51 \
    --chart fabedge/fabedge
```

Parameter description:

- connector-public-addresse: the public address of connectors and it should be accessible from edge nodes.
- connector-public-port and connector-as-mediator: both are not required for HA deployment, they are here because it's necessary for this enviroment.
- enable-keepalived： whether to use builtin keepalived, it is enabled in this example.
- keepalived-vip： the virtual IP for connector and it should be an internal IP.
- keepalived-interface： the interface which is used to assign virtual IP and make sure all interfaces on connector nodes share the same name.
- keepalived-router-id： same as keepalived virtual_router_id which is used to identify different vrrp instances, not required.

4. Check if FabEdge is deployed successfully：

```shell
root@harry:~/fabedge# kubectl get nodes -o wide
NAME    STATUS   ROLES                  AGE     VERSION                    INTERNAL-IP    EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION      CONTAINER-RUNTIME
edge1   Ready    agent,edge             7d      v1.22.6-kubeedge-v1.12.2   10.22.53.116   <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://20.10.21
edge4   Ready    agent,edge             7d1h    v1.22.6-kubeedge-v1.12.2   10.40.30.110   <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://24.0.5
harry   Ready    control-plane,master   21d     v1.22.5                    192.168.1.5    <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://24.0.5
node1   Ready    connector              21d     v1.22.6                    192.168.1.6    <none>        Ubuntu 20.04.6 LTS   5.4.0-167-generic   docker://24.0.5
node2   Ready    connector              4h55m   v1.22.5                    192.168.1.7    <none>        Ubuntu 20.04.6 LTS   5.4.0-166-generic   docker://24.0.5

root@harry:~/fabedge# kubectl get po -o wide
NAME                                READY   STATUS    RESTARTS   AGE     IP             NODE    NOMINATED NODE   READINESS GATES
fabedge-agent-55fnj                 2/2     Running   0          7m14s   10.22.53.116   edge1   <none>           <none>
fabedge-agent-vvwdz                 2/2     Running   0          7m14s   10.40.30.110   edge4   <none>           <none>
fabedge-cloud-agent-rcwqk           1/1     Running   0          7m16s   192.168.1.5    harry   <none>           <none>
fabedge-connector-7b659c4cd-475l2   3/3     Running   0          7m14s   192.168.1.7    node2   <none>           <none>
fabedge-connector-7b659c4cd-6tj2c   3/3     Running   0          7m14s   192.168.1.6    node1   <none>           <none>
fabedge-operator-5f4c5b5ffd-6cghs   1/1     Running   0          7m16s   10.233.66.21   node2   <none>           <none>

root@harry:~/fabedge# kubectl get lease
NAME        HOLDER   AGE
connector   node2    7m43s
```

 here are two connectors pods running, both will try to acquire connector lease and the one who get the lease will function as connector, while the other one will function as cloud-agent untill it acquire connector lease.       

## Manually Deploy

We can also deploy HA FabEdge, please read [Manually Deploy](./manually-install.md) first. We won't repeat  those steps here, just provide an example of values.yaml:

```yaml
cluster:
  name: harry
  role: host
  region: harry
  zone: harry

  cniType: "flannel"
  
  clusterCIDR:
  - 10.233.64.0/18
  connectorPublicAddresses:
  - 10.40.10.180
  connectorPublicPort: 45000
  connectorAsMediator: true
  serviceClusterIPRange:
  - 10.233.0.0/18

connector:
  replicas: 2

keepalived:
  create: true
  interface: enp0s3
  routerID: 51
  vip: 192.168.1.200

agent:
  args:
    ENABLE_PROXY: "true" 
    ENABLE_DNS: "true" 
```