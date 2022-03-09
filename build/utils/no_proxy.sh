#!/bin/bash

# remove China stuff to speedup build on github

set -ex

for app in agent connector operator strongswan
do
   sed -i 's/gitee/github/g' build/$app/Dockerfile
   sed -i 's/GOPROXY/NOGOPROXY/g' build/$app/Dockerfile
   sed -i 's/dl-cdn.alpinelinux.org/nothing-to-do/g' build/$app/Dockerfile
done
