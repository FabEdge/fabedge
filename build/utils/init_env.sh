#!/bin/sh

set -x

find /etc/cni/net.d/ -type f -not -name fabedge.conf -exec rm {} \;
cp -f /usr/local/bin/bridge /usr/local/bin/host-local /usr/local/bin/loopback /opt/cni/bin

ip link delete cni0
ip link delete flannel.1
ip route | grep "flannel" |  while read dst via gw others; do ip route delete $dst via $gw; done

exit