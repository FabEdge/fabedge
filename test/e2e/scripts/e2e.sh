#!/bin/bash

# how to compile the fabedge-e2e.test binary
# git pull xxx
# make e2e-test
# cd _output/
# scp fabedge-e2e.test 10.20.8.24:~

# CLUSTER_IPS_PATH=./cluster-master-ips
CLUSTER_IPS_PATH=""
# MULTI_CLUSTER_KUBECONFIG_STORE_DIR=/tmp/e2ekubeconfigs
MULTI_CLUSTER_KUBECONFIG_STORE_DIR=""
KUBECONFIG_DEFAULT_PATH=/root/.kube/config
TIMEOUT=300
# EDGE_LABELS=""

function usage() {
  echo "USAGE:"
  echo "  prepare-kubeconfig  [clusters_kubeconfig_store_dir] [cluster_ip_list_file_path]
                      e.g. prepare-kubeconfigs /tmp/e2ekubeconfigs ./cluster-master-ips
        "
  echo "  multi-cluster       [clusters_kubeconfig_store_dir]
                      e.g. multi-cluster /tmp/e2ekubeconfigs
        "
  exit 0
}

function read_ip_list_file() {
  while read line
  do
    #masterIP=`echo $line |sed 's/^\s*//' |sed 's/\s*$//'`
    #echo $masterIP
    echo "cluster IP <$line> copy kube-config"
    scp root@$line:$KUBECONFIG_DEFAULT_PATH $MULTI_CLUSTER_KUBECONFIG_STORE_DIR/$line

  done < "$1"
}

function multi_cluster_kubeconfig_prepare() {
  if [ ! -f "$CLUSTER_IPS_PATH" ];then
    echo "$CLUSTER_IPS_PATH file is needed for noting the IPs of clusters to do e2e process."
    exit 1
  fi

  rm -rf $MULTI_CLUSTER_KUBECONFIG_STORE_DIR
  mkdir -p $MULTI_CLUSTER_KUBECONFIG_STORE_DIR

  # get all clusters kubeconfig files
  read_ip_list_file $CLUSTER_IPS_PATH
  echo "prepare multi-cluster kubeconfig done."
}

function exec_test () {
  if [ -n "$MULTI_CLUSTER_KUBECONFIG_STORE_DIR" ];
  then
    ./fabedge-e2e.test \
      -multi-cluster-kube-config-dir="$MULTI_CLUSTER_KUBECONFIG_STORE_DIR" \
      -wait-timeout="$TIMEOUT" \
      -ping-timeout="$TIMEOUT" \
      -curl-timeout="$TIMEOUT" \
      -preserve-resources=fail \
      -show-exec-error=true
      # -edge-labels="$EDGE_LABELS"
  else
    ./fabedge-e2e.test \
      -wait-timeout="$TIMEOUT" \
      -ping-timeout="$TIMEOUT" \
      -curl-timeout="$TIMEOUT" \
      -preserve-resources=fail \
      -show-exec-error=true
      # -edge-labels="$EDGE_LABELS"
  fi
}

case $1 in
  "-h"|"--help")
    usage
  ;;
  "prepare-kubeconfig")
    if [ $# -ne 3 ]; then
      usage
    fi
    MULTI_CLUSTER_KUBECONFIG_STORE_DIR="$2"
    CLUSTER_IPS_PATH="$3"
    multi_cluster_kubeconfig_prepare
  ;;
  "multi-cluster")
    if [ $# -lt 2 ]; then
      usage
    fi
    MULTI_CLUSTER_KUBECONFIG_STORE_DIR="$2"
    if [ $# -eq 3 ] && [ -z "$(echo $3 | sed 's/[0-9]//g')" ] && [ "$3" -gt 0 ]; then
      TIMEOUT="$3"
    fi
    exec_test
  ;;
  *)
    if [ $# -eq 1 ] && [ -z "$(echo $1 | sed 's/[0-9]//g')" ] && [ "$1" -gt 0 ]; then
      TIMEOUT=$1
    fi
    exec_test
  ;;
esac

