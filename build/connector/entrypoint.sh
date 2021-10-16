#!/bin/sh

set -e

function is_nft_loaded
{
  lsmod | grep table | grep nft >/dev/null
}

if is_nft_loaded; then
   echo "entrypoint:    use iptables of nft mode"
   ln -sf /sbin/xtables-nft-multi /sbin/iptables
else
   echo "entrypoint:    use iptables of legacy mode"
fi

cmd="/usr/local/bin/connector $@"
echo "entrypoint:    run command: $cmd"

exec $cmd
