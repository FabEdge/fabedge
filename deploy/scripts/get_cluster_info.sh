#!/bin/bash

echo "This may take some time. Please wait."
echo ""

cluster_dns=$(kubectl get cm nodelocaldns -n kube-system -o jsonpath="{.data.Corefile}" 2> /dev/null | awk '/bind/ && !a[$0]++{print $2}')
cluster_name=$(grep -r cluster-name /etc/kubernetes/ | awk -F '=' 'END{print $NF}')
cluster_cidr=$(grep -r cluster-cidr /etc/kubernetes/ | awk -F '=' 'END{print $NF}')
service_cluster_ip_rang=$(grep -r service-cluster-ip-range /etc/kubernetes/ | awk -F '=' 'END{print $NF}')


if [ x"$cluster_name" = x"" -o x"$cluster_cidr" = x"" -o x"$service_cluster_ip_rang" = x"" ];
then
    while read line
    do
        if [[ "$line" =~ \"--cluster-cidr=.* ]];
        then
            cluster_cidr=`awk -F '["=]' '{print $3}' <<< $line`
        elif [[ "$line" =~ \"--cluster-name=.* ]];
        then
            cluster_name=`awk -F '["=]' '{print $3}' <<< $line`
        elif [[ "$line" =~ \"--service-cluster-ip-range=.* ]];
        then
            service_cluster_ip_rang=`awk -F '["=]' '{print $3}' <<< $line`
        fi
    done <<< "`kubectl cluster-info dump | awk '(/cluster-cidr/ || /cluster-name/ || /service-cluster-ip-range/) && !a[$0]++{print}'`"
fi


echo "clusterDNS               : $cluster_dns"
echo "clusterDomain            : $cluster_name"
echo "cluster-cidr             : $cluster_cidr"
echo "service-cluster-ip-range : $service_cluster_ip_rang"
