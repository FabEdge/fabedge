# FabEdge troubleshooting manual

[toc]

## Verify that the Kubernetes environment is normal

```shell
kubectl get po -n kube-system
kubectl get no 
```

If Kubernetes is not working, please do your own troubleshooting until the problem is resolved, and then the next step.  


## Confirm FabEdge service is normal

If the FabEdge service is abnormal, check related logs.

```shell
# Execute on master, use the correct pod name.
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

## Verify that the tunnel is successfully established

```shell
# Execute on master.
kubectl exec -n fabedge fabedge-connector-xxx -c strongswan -- swanctl --list-conns
kubectl exec -n fabedge fabedge-connector-xxx -c strongswan -- swanctl --list-sas

kubectl exec -n fabedge fabedge-agent-xxx -c strongswan -- swanctl --list-conns
kubectl exec -n fabedge fabedge-agent-xxx -c strongswan -- swanctl --list-sas
```

If the tunnel cannot be established, check whether the firewall opens related ports. For details, see the installation manual.

## Check the routing table 

```shell
# Run on the connector node.
ip l
ip r
ip r s t 220
ip x p 
ip x s

# Run on edge nodes
ip l
ip r
ip r s t 220
ip x p 
ip x s

# Run on non-connector nodes in the cloud  
ip l
ip r
```

If **edge node**  has interfaces such as CNI, flannel residues exist and you need to restart **edge node**.  

## Check the iptables

```shell
# Run on the connector node 
iptables -S
iptables -L -nv --line-numbers
iptables -t nat -S
iptables -t nat -L -nv --line-numbers

# Run on edge nodes.
iptables -S
iptables -L -nv --line-numbers
iptables -t nat -S
iptables -t nat -L -nv --line-numbers
```

Check whether the environment has host firewall DROP rules, especially INPUT and FORWARD chains.

## Screening tool

You can also use the script below to quickly collect the above information. If you need community support, please submit the generated file. 

```
# The master node executes：
curl http://116.62.127.76/checker.sh | bash -s master | tee /tmp/master-checker.log

# Connector node execute：
curl http://116.62.127.76/checker.sh | bash -s connector | tee /tmp/connector-checker.log

# Edge node execute：
curl http://116.62.127.76/checker.sh | bash -s edge | tee /tmp/edge-checker.log

# Other nodes execute：
curl http://116.62.127.76/checker.sh | bash | tee /tmp/node-checker.log
```