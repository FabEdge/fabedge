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

function echoBanner() {
  name=$1
  color=$2
  nameLength=$(echo $name | wc -L)
  shellWidth=`stty size|awk '{print $2}'`
  outWord='-'
  outWordSize=$(expr $shellWidth - $nameLength)
  outWordLeftSize=$(expr $outWordSize / 2)
  outWordRightSize=$(expr $shellWidth - $nameLength - $outWordLeftSize)

  banner=$(printf %${outWordLeftSize}s |tr " " "${outWord}"; echo -n $name; printf %${outWordRightSize}s |tr " " "${outWord}";echo)
  if [[ $color == green ]];then
    echoGreen $banner
  elif [[ $color == red ]]; then
    echoRed $banner
  else
    echo $banner
  fi
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
  echoBlue "$ kubectl get nodes -o wide"

  nodesInfo=$(kubectl get nodes -o wide)
  exitCode=$?
  failed="false"

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
      failed="true"
    fi
  done <<< "$nodesInfo"
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
  echoBlue "$ kubectl get pods -n kube-system -o wide"
  podsInfo=$(kubectl get pods -n kube-system -o wide)
  exitCode=$?
  checkPods kube-system
}

function checkFabedgePods () {
  echoBlue "$ kubectl get pods -n $FABEDGE_NAMESPACE -o wide"
  podsInfo=$(kubectl get pods -n $FABEDGE_NAMESPACE -o wide)
  exitCode=$?
  checkPods $FABEDGE_NAMESPACE
}

function printStrongswanConnsAndSas () {
  podsInfo=$(kubectl get pods -n $FABEDGE_NAMESPACE)
  exitCode=$?
  if [ $exitCode != 0 ]; then
    return 1
  fi
  while read name ready status restarts age; do
    if [[ $name =~ ^fabedge ]];then
      printColorOutput "kubectl -n $FABEDGE_NAMESPACE describe pods $name"

      if [[ $name =~ ^fabedge-agent|^fabedge-connector ]];then
        printColorOutput "kubectl exec -n $FABEDGE_NAMESPACE $name -c strongswan -- swanctl --list-conns"
        printColorOutput "kubectl exec -n $FABEDGE_NAMESPACE $name -c strongswan -- swanctl --list-sa"
      fi

      for containerName in $(kubectl -n $FABEDGE_NAMESPACE get pods $name -o jsonpath='{.spec.initContainers[*].name} {.spec.containers[*].name}');
      do
        printColorOutput "kubectl logs --tail 50 -n $FABEDGE_NAMESPACE $name -c $containerName"
      done
    fi
  done <<< "$podsInfo"
}

function printIPRouteInfo() {
  printColorOutput "ip l"
  printColorOutput "ip r"
  printColorOutput "ip r s t 220"
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
    printStrongswanConnsAndSas
    printIPRouteInfo
    ;;
  connector)
    printIPRouteInfo
    printIptablesInfo
    ;;
  edge)
    printIPRouteInfo
    printIptablesInfo
    ;;
  *)
    printIPRouteInfo
    ;;
esac
