#!/bin/bash

FABEDGE_NAMESPACE=${FABEDGE_NAMESPACE:-fabedge}


function echoGreen() {
    echo -e "\033[32m$@ \033[0m"
}
function echoRed() {
   echo -e "\033[31m$@ \033[0m"
}
function echoBlue() {
   echo -e "\033[34m$@ \033[0m"
}

function printColorOutput() {
  command=$1
  keyworld=${2:-"********************"}

  echoBlue "$ $command"

  while read line;
  do
    if [[ $line =~ $keyworld ]];
    then
      echoRed "$line"
    else
      echo "$line"
    fi
  done <<< "$($command)"
  echo
}

function checkNodes () {
  command="kubectl get nodes -o wide"
  echoBlue "$ $command"

  while read line; do
    read name status roles age version <<< $line
    if [[ $name == NAME ]]; then
      echo "$line"
      continue
    fi

    if [[ $status == Ready ]]; then
      echo "$line"
    else
      echoRed "$line"
    fi
  done <<< "$($command)"
}

function checkPods () {
  while read line; do
    read name ready status restarts age <<< $line
    if [[ $name == NAME ]]; then
      echo "$line"
      continue
    fi
    read readyContainers totalContainers <<< $(awk -F '/' '{print $1, $2}' <<< $ready)
    if [ $readyContainers != $totalContainers -o $status != Running ];then
      echoRed "$line"
      continue
    fi
    echo "$line"
  done <<< "$podsInfo"
}

function checkKubeSystemPods () {
  command="kubectl get pods -n kube-system -o wide"
  echoBlue "$ $command"
  podsInfo=$($command)
  checkPods kube-system
}

function checkFabedgePods () {
  command="kubectl get pods -n $FABEDGE_NAMESPACE -o wide"
  echoBlue "$ $command"
  podsInfo=$($command)
  checkPods $FABEDGE_NAMESPACE
}

function printFabedgeDetails () {
  podsInfo=$(kubectl get pods -n $FABEDGE_NAMESPACE)

  while read name ready status restarts age; do
    printColorOutput "kubectl -n $FABEDGE_NAMESPACE describe pods $name"

    if [[ $name =~ agent|connector ]];then
      printColorOutput "kubectl exec -n $FABEDGE_NAMESPACE $name -c strongswan -- swanctl --list-conns"
      printColorOutput "kubectl exec -n $FABEDGE_NAMESPACE $name -c strongswan -- swanctl --list-sa"
    fi

    for containerName in $(kubectl -n $FABEDGE_NAMESPACE get pods $name -o jsonpath='{.spec.initContainers[*].name} {.spec.containers[*].name}');
    do
      printColorOutput "kubectl logs --tail 50 -n $FABEDGE_NAMESPACE $name -c $containerName"
    done
  done <<< "$podsInfo"
}

function printIPRouteInfo() {
  printColorOutput "ip l"
  printColorOutput "ip r"
  printColorOutput "ip r s t 220"
}

function printXFRMInfo() {
  printColorOutput "ip x p"
  printColorOutput "ip x s"
}

function printIptablesInfo() {
  printColorOutput "iptables -S" DROP
  printColorOutput "iptables -L -nv --line-numbers" DROP
  printColorOutput "iptables -t nat -S" DROP
  printColorOutput "iptables -t nat -L -nv --line-numbers" DROP
}

function printSystemInfo() {
  primaryIP=$(ip route get 8.8.8.8 | awk '{for(i=1;i<=NF;i++){if ($i=="src"){print $(i+1); exit}}}')
  echo
  echo "hostname: $(hostname) ip: ${primaryIP}"
  echo
}

printSystemInfo

case ${1} in
  master)
    checkNodes
    checkKubeSystemPods
    checkFabedgePods
    printFabedgeDetails
    printIPRouteInfo
    ;;
  connector)
    printIPRouteInfo
    printXFRMInfo
    printIptablesInfo
    ;;
  edge)
    printIPRouteInfo
    printXFRMInfo
    printIptablesInfo
    ;;
  *)
    printIPRouteInfo
    ;;
esac
