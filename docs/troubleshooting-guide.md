# FabEdge troubleshooting manual

[toc]

English | [中文](troubleshooting-guide_zh.md)

## Verify that the Kubernetes environment is normal

```shell
kubectl get po -n kube-system
kubectl get no 
```


## Confirm FabEdge service is normal

If the FabEdge service is abnormal, check related logs.

```shell
# Execute on master node, use the correct pod name.
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
# Execute on master node.
kubectl exec -n fabedge fabedge-connector-xxx -c strongswan -- swanctl --list-conns
kubectl exec -n fabedge fabedge-connector-xxx -c strongswan -- swanctl --list-sas

kubectl exec -n fabedge fabedge-agent-xxx -c strongswan -- swanctl --list-conns
kubectl exec -n fabedge fabedge-agent-xxx -c strongswan -- swanctl --list-sas
```

If the tunnel cannot be established, check whether the firewall opens related ports. For details, see the  [install](install.md).

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

> Note: If **edge node**  has interfaces such as CNI, means flannel residues exist and you need to restart **edge node**.  

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



## Verify the certificate 

FabEdge related certificates including CA, Connector, and Agent, are stored in Secret and maintained by Operator automatically. If a certificate-related error occurs, you can use the following method to manually verify.  

```shell
# Execute on the master node.

# Start a container for cert.
docker run fabedge/cert

# Get the ID of the container you just started.  
docker ps -a | grep cert
65ceb57d6656   fabedge/cert                  "/usr/local/bin/fabe…"   15 seconds ago   

# Copy the executable to the host.
docker cp 65ceb57d6656:/usr/local/bin/fabedge-cert .

# Check out related secret.  
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

# Verify related secret.  
./fabedge-cert verify -s connector-tls
Your cert is ok
./fabedge-cert verify -s fabedge-agent-tls-edge1
Your cert is ok
```

## Screening tool

You can use the script below to quickly collect the above information. If you need community support, please submit the generated file. 

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