#!/bin/bash

# remove China stuff to speedup build on github

set -ex

for app in agent connector operator strongswan
do
   sed -i 's/GOPROXY/NOGOPROXY/g' build/$app/Dockerfile
   sed -i 's/dl-cdn.alpinelinux.org/nothing-to-do/g' build/$app/Dockerfile
done

sed -i 's#-i https://pypi.tuna.tsinghua.edu.cn/simple/##g' build/installer/Dockerfile
