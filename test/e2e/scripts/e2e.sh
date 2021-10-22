#!/bin/bash

# how to compile the fabedge-e2e.test binary
# git pull xxx
# make e2e-test
# cd _output/
# scp fabedge-e2e.test 10.20.8.24:~

kubectl delete community all-edge-nodes
kubectl delete ns fabedge-e2e-test

# edge-labels is used to select edge nodes
# kubedge: node-role.kubernetes.io/edge
# superedge: superedge.io/edge-node=enable
# openyurt: openyurt.io/is-edge-worker=true

./fabedge-e2e.test -wait-timeout 600 -ping-timeout 300 -curl-timeout 300 --edge-labels "node-role.kubernetes.io/edge"
