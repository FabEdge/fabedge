#!/bin/bash
set -e

VERSION="v0.4.beta"
ARCH="amd64"

_images="fabedge/cloud-agent:${VERSION} fabedge/strongswan:5.9.1 fabedge/operator:${VERSION} fabedge/connector:${VERSION} fabedge/agent:${VERSION} fabedge/cert:${VERSION}"
_os="linux"

if [[ $ARCH == "arm64" ]];then
  _variant="v8"
elif [[ $ARCH == "arm" ]];then
  _variant="v7"
else
  _variant=null
fi


for image in ${_images}; do
    manifestFilename=/tmp/manifest_${image#*/}.json
    docker manifest inspect $image > ${manifestFilename}
    manifests_length=$(jq '.manifests | length' ${manifestFilename})
    manifests_end=$(expr $manifests_length - 1)
    
    for ID in $(seq 0 $manifests_end);
    do
        digest=$(jq ".manifests[${ID}].digest"                      ${manifestFilename} | sed 's/\"//g')
        architecture=$(jq ".manifests[${ID}].platform.architecture" ${manifestFilename} | sed 's/\"//g')
        os=$(jq ".manifests[${ID}].platform.os"                     ${manifestFilename} | sed 's/\"//g')
        variant=$(jq ".manifests[${ID}].platform.variant"           ${manifestFilename} | sed 's/\"//g')

        if [[ $ARCH != $architecture ]]; then
            continue
        fi
        if [[ $_os != $os ]]; then
            continue
        fi
        if [[ $_variant != $variant ]]; then
            continue
        fi

        docker pull ${image}@${digest}
        docker tag ${image}@${digest} ${image}
    done
done

docker save ${_images} -o fabedge-linux-${ARCH}-${VERSION}.images
