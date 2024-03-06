#!/bin/sh
set -e

echo 'select between the two modes of iptables ("legacy" and "nft")'
/iptables-wrapper-installer.sh

cmd="/usr/local/bin/node $@"
echo "entrypoint:    run command: $cmd"

exec $cmd
