#!/bin/bash

kubectl delete community all-edge-nodes
kubectl delete ns fabedge-e2e-test

# edge-labels:
# kubedge: node-role.kubernetes.io/connector
# superedge: superedge.io/edge-node=enable
# openyurt: openyurt.io/is-edge-worker=true

./fabedge-e2e.test -wait-timeout 600 -ping-timeout 300 -curl-timeout 300 --edge-labels "node-role.kubernetes.io/connector"
