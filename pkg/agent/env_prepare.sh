#!/bin/sh

set -x

# install CNI plugins
find /etc/cni/net.d/ -type f -not -name fabedge.conf -exec rm {} \;
cp -f /usr/local/bin/bridge /usr/local/bin/host-local /usr/local/bin/loopback /opt/cni/bin

# cleanup flannel stuff
ip link delete cni0
ip link delete flannel.1
ip route | grep "flannel" |  while read dst via gw others; do ip route delete $dst via $gw; done
iptables -t nat -F POSTROUTING

# cleanup xfrm stuff
ip xfrm policy flush
ip xfrm state flush

exit 0
