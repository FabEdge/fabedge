# FabEdge排错手册

[toc]

## 确认Kubernetes环境正常

```shell
kubectl get po -n kube-system
kubectl get no 
```

如果Kubernetes不正常，请自行排查，直到问题解决，然后下一步


## 确认FabEdge服务正常

如果FabEdge服务不正常，检查相关日志

```shell
# 在master节点上执行, 请使用正确的Pod的名字
kubectl get po -n fabedge

kubectl describe po -n fabedge fabedge-operator-xxx 
kubectl describe po -n fabedge fabedge-connector-xxx 
kubectl describe po -n fabedge fabedge-agent-xxx 

kubectl logs --tail=50 -n fabedge fabedge-operator-5fc5c4b56-glgjh

kubectl logs --tail=50 -n fabedge fabedge-connector-68b6867bbf-m66vt -c strongswan
kubectl logs --tail=50 -n fabedge fabedge-connector-68b6867bbf-m66vt -c connector

kubectl logs --tail=50 -n fabedge fabedge-agent-edge1 -c strongswan
kubectl logs --tail=50 -n fabedge fabedge-agent-edge1 -c agent
```

## 确认隧道建立成功

```shell
# 在master节点上执行
kubectl exec -n fabedge fabedge-connector-xxx -c strongswan -- swanctl --list-conns
kubectl exec -n fabedge fabedge-connector-xxx -c strongswan -- swanctl --list-sas

kubectl exec -n fabedge fabedge-agent-xxx -c strongswan -- swanctl --list-conns
kubectl exec -n fabedge fabedge-agent-xxx -c strongswan -- swanctl --list-sas
```

如果隧道不能建立，要确认防火墙是否开放相关端口，具体参考[安装手册](get-started.md)

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

## 校验证书

FabEdge相关的证书，包括CA， Connector， Agent都保存在Secret里，Operator会自动维护这些证书。如果出现证书相关错误，可以使用下面方法手动验证。

```shell
# 在master节点上执行

# 启动一个cert的容器
docker run fabedge/cert

# 获取刚启动的容器的ID 
docker ps -a | grep cert
65ceb57d6656   fabedge/cert                  "/usr/local/bin/fabe…"   15 seconds ago   

# 将可执行程序拷贝到主机
docker cp 65ceb57d6656:/usr/local/bin/fabedge-cert .

# 查看相关Secret
kubectl get secret -n fabedge
NAME                            TYPE                                  DATA   AGE
api-server-tls                  kubernetes.io/tls                     4      3d22h
cert-token-csffn                kubernetes.io/service-account-token   3      3d22h
connector-tls                   kubernetes.io/tls                     4      3d22h
default-token-rq9mv             kubernetes.io/service-account-token   3      3d22h
fabedge-agent-tls-edge1         kubernetes.io/tls                     4      3d22h
fabedge-agent-tls-edge2         kubernetes.io/tls                     4      3d22h
fabedge-ca                      Opaque                                2      3d22h
fabedge-operator-token-tb8qb    kubernetes.io/service-account-token   3      3d22h

# 校验相关Secret
./fabedge-cert verify -s connector-tls
Your cert is ok
./fabedge-cert verify -s fabedge-agent-tls-edge1
Your cert is ok
```



## 排查工具

也可以使用下面的脚本快速收集以上信息，如需社区提供支持，请提交生成的文件。

```
# master节点执行：
curl http://116.62.127.76/checker.sh | bash -s master | tee /tmp/master-checker.log

# connector节点执行：
curl http://116.62.127.76/checker.sh | bash -s connector | tee /tmp/connector-checker.log

# edge节点执行：
curl http://116.62.127.76/checker.sh | bash -s edge | tee /tmp/edge-checker.log

# 其它节点执行：
curl http://116.62.127.76/checker.sh | bash | tee /tmp/node-checker.log
```