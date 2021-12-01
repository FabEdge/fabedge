# FabEdge排错手册

[toc]

## 确认Kubernetes环境正常

```shell
kubectl get po -n kube-system
kubectl get no 
```

如果Kubernetes不正常，请自行排查，直到问题解决。



## 确认FabEdge服务正常

如果FabEdge服务不正常，检查相关日志

```shell
# 在master上执行
kubectl get po -n fabedge

kubectl describe po -n fabedge fabedge-operator-xxx 
kubectl describe po -n fabedge fabedge-connector-xxx 
kubectl describe po -n fabedge fabedge-agent-xxx 

kubectl logs --tail=50 -n fabedge fabedge-operator-xxx 
kubectl logs --tail=50 -n fabedge fabedge-connector-xxx 
kubectl logs --tail=50 -n fabedge fabedge-agent-xxx 
```



## 确认隧道建立成功

```shell
# 在master上执行
kubectl exec -n fabedge fabedge-connector-xxx -c strongswan -- swanctl --list-conns
kubectl exec -n fabedge fabedge-connector-xxx -c strongswan -- swanctl --list-sas

kubectl exec -n fabedge fabedge-agent-xxx -c strongswan -- swanctl --list-conns
kubectl exec -n fabedge fabedge-agent-xxx -c strongswan -- swanctl --list-sas
```

如果隧道不能建立，要确认防火墙是否开放相关端口，具体参考安装手册



## 检查路由表

```shell
# 在connector节点上运行
ip l
ip r
ip r s t 220
ip x p 
ip x s

# 在边缘节点上运行
ip l
ip r
ip r s t 220
ip x p 
ip x s

# 在云端非connector节点上运行
ip l
ip r
```

如果**边缘节点**有cni等接口，表示有flannel的残留，需要重启**边缘节点**



## 检查iptables

```shell
# 在connector节点上运行
iptables -S
iptables -L -nv --line-numbers
iptables -t nat -S
iptables -t nat -L -nv --line-numbers

# 在边缘节点上运行
iptables -S
iptables -L -nv --line-numbers
iptables -t nat -S
iptables -t nat -L -nv --line-numbers
```

检查是否环境里有主机防火墙DROP的规则，尤其是INPUT， FORWARD的链



## 排查工具

TODO