#!/bin/bash
set -e

CONTAINER_MANAGER=${CONTAINER_MANAGER:-docker}
CONTAINER_NAME=installer
IMAGE_NAME=fabedge/installer:latest
EXTRA_VARS="container_manager=${CONTAINER_MANAGER} ${EXTRA_VARS}"
HOST_VARS=$@


function info() {
    echo '[INFO] ' "$@"
}
function warn() {
    echo '[WARN] ' "$@" >&2
}
function fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

function validate_container_manager() {
    case $CONTAINER_MANAGER in
        docker)
            :
            ;;
        *)
            fatal "unsupported CONTAINER_MANAGER: ${CONTAINER_MANAGER}"
            ;;
    esac
}


function docker_add_edge() {
    docker restart ${CONTAINER_NAME} > /dev/null
    docker exec ${CONTAINER_NAME} bash -c "( DEBUG=False /usr/bin/python /opt/deploy.py --job-chain edge-deploy.yaml --skip-deployed-job false --limit '!master*' --extra-vars ${EXTRA_VARS} ${HOST_VARS} )"
}

function add_edge() {
    if [ "$CONTAINER_MANAGER" = "docker" ];then
        docker_add_edge
    fi
}


validate_container_manager

add_edge
