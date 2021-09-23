#!/bin/bash

echo "This may take some time. Please wait."
echo ""

while read line
do
    if [[ "$line" =~ \"--cluster-name=.* ]];
    then
        cluster_name=`awk -F '["=]' '{print $3}' <<< $line`
    elif [[ "$line" =~ \"--service-cluster-ip-range=.* ]];
    then
        service_cluster_ip_rang=`awk -F '["=]' '{print $3}' <<< $line`
    fi
done <<< "`kubectl cluster-info dump | awk '(/cluster-cidr/ || /cluster-name/ || /service-cluster-ip-range/) && !a[$0]++{print}'`"


cluster_dns=$(kubectl get cm nodelocaldns -n kube-system -o jsonpath="{.data.Corefile}" 2> /dev/null | awk '/bind/ && !a[$0]++{print $2}')

echo "clusterDNS               : $cluster_dns"
echo "clusterDomain            : $cluster_name"
echo "service-cluster-ip-range : $service_cluster_ip_rang"
