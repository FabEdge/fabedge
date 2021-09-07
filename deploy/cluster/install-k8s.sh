#!/bin/bash
set -e

CONTAINER_MANAGER=${CONTAINER_MANAGER:-docker}
CONTAINER_NAME=installer
IMAGE_NAME=fabedge/installer:latest
EXTRA_VARS="container_manager=${CONTAINER_MANAGER} ${EXTRA_VARS}"
HOST_VARS=$@

### ID=ubuntu VERSION_ID="20.04" ID="centos" VERSION_ID="7"
OS_ID=`awk -F '=' '/^ID=/{gsub(/"/,""); print $2}' /etc/os-release`
OS_VERSION_ID=`awk -F '"' '/^VERSION_ID=/{print $2}' /etc/os-release`

ID_RSA_PUB='ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDkYx9ASmLpDvEE6Ap4wsvBP/Sspol9ic7Ow7N5ARz+SR3noqELb5MSRXK0povMtj5NMg96cRkeH1PBiKfR5kzm6rY3AQ/Ph7qKTaK+iDxHHuEMTkHh9iW+7BEpTKT94juXg7f1JK+Q83kBMN0UHxTzxKpnJIzv1j6P/bSR7NYoKvDDT8VE/ep9axmrn3jkPxyssRkDmqYqyb5fp8kJbT4Uny7Wsc8i/ObEQcD//qvH9X/OVZ3rE66xchFs8B5WhYq3FR0/Ne6nRjPoDWxAdAnFuSbaoviDLmpQXbA4xdN6sN2du3NXRF4k3gygfn+b/0fvL1ii1Sr3NjLX0fRlbFjX'

releases_directory='/opt/releases'
RELEASES_URL='http://116.62.127.76/kubespray-v2.15.0-x86_64/releases/'
#RELEASES_URL='http://10.22.1.4:10120/fabedge/kubespray-v2.15.0-x86_64/releases/'
images_directory='/opt/images'
IMAGES_URL='http://116.62.127.76/'
#IMAGES_URL='http://10.22.1.4:10120/fabedge/'
mkdir -p ${releases_directory} ${images_directory} || true

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

function install_ssh_key() {
    if [ ! -d "${HOME}/.ssh/" ]; then
        mkdir "${HOME}/.ssh/"
    fi
    touch ${HOME}/.ssh/authorized_keys
    grep -q "${ID_RSA_PUB}" ${HOME}/.ssh/authorized_keys || echo "${ID_RSA_PUB}" >> ${HOME}/.ssh/authorized_keys
}


function validate_os_id() {
    if [ "$OS_ID" == "centos" ]; then
        if [ "$OS_VERSION_ID" != "7" -a "$OS_VERSION_ID" != "8" ]; then
            fatal "unsupported os-version: ${OS_ID}-${OS_VERSION_ID}"
        fi
    elif [ "$OS_ID" == "ubuntu" ]; then
        if [ "$OS_VERSION_ID" != "18.04" -a "$OS_VERSION_ID" != "20.04" ]; then
            fatal "unsupported os-version: ${OS_ID}-${OS_VERSION_ID}"
        fi
    else
        fatal "unsupported os: $OS_ID"
    fi
}

function install_tools() {
    if [ "$OS_ID" == "centos" ]; then
        yum -y install wget curl
    elif [ "$OS_ID" == "ubuntu" ]; then
        apt-get -y install wget curl
    else
        fatal "unsupported os: $OS_ID"
    fi
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

function docker_install() {
    if docker --help > /dev/null 2>&1; then
        return
    fi
    curl -fsSL https://get.docker.com | bash -s docker --mirror Aliyun
    systemctl restart docker
    systemctl enable docker
}

function docker_run() {
    if [ ! $(docker ps -q --filter name="${CONTAINER_NAME}") ]; then
        docker run --restart always --privileged --init --net host -d --name ${CONTAINER_NAME} ${IMAGE_NAME} sleep infinity
    fi
}

function docker_cp() {
    docker cp $*
}

function docker_load() {
    for image in $*; do
        docker load -i ${images_directory}/${image}
    done
}

function docker_deploy() {
    docker restart ${CONTAINER_NAME} > /dev/null
    docker exec ${CONTAINER_NAME} bash -c "( DEBUG=False /usr/bin/python /opt/deploy.py --job-chain deploy.yaml --extra-vars ${EXTRA_VARS} ${HOST_VARS} )"
    #if [ x"$HOST_VARS" != x"" ]; then
    #    docker exec ${CONTAINER_NAME} bash -c "( /usr/bin/python /opt/deploy.py --job-chain edge-deploy.yaml --skip-deployed-job false --extra-vars ${EXTRA_VARS} ${HOST_VARS} )"
    #fi
}

function install_container_manager() {
    if [ "$CONTAINER_MANAGER" == "docker" ]; then
        docker_install
    fi
}

function deploy() {
    if [ "$CONTAINER_MANAGER" = "docker" ];then
        docker_deploy
    fi
}

function run() {
    if [ "$CONTAINER_MANAGER" = "docker" ];then
        docker_run
    fi
}

function cp_() {
    if [ "$CONTAINER_MANAGER" = "docker" ];then
        docker_cp $*
    fi
}

function load() {
    if [ "$CONTAINER_MANAGER" = "docker" ];then
        docker_load $*
    fi
}

download_images() {
    for image in $*; do
	if [ -f "${images_directory}/${image}" ]; then
	    continue
	fi
        if ! wget ${IMAGES_URL}${image} -O ${images_directory}/${image}; then
            rm -f ${images_directory}/${image}
	    fatal "Failed to download image: ${IMAGES_URL}${image}"
	fi
    done
}

download_releases() {
    for binary in $*; do
        if [ -f "${releases_directory}/${binary}" ]; then
	    continue
	fi
        if ! wget --preserve-permissions ${RELEASES_URL}${binary} -O ${releases_directory}/${binary}; then
	    rm -f ${releases_directory}/${binary}
	    fatal "Failed to download binary: ${RELEASES_URL}${binary}"
	fi
	chmod 0755 ${releases_directory}/${binary}
    done

}

# validate_os_id
validate_container_manager
install_ssh_key
install_tools
install_container_manager

download_images cloudcore-v1.5.0-x86_64.tar kubespray-v2.15.0-x86_64.tar kubeedge-pause-3.1-x86_64.tar k8s-dns-node-cache-1.16.0-x86_64.tar
download_releases calicoctl kubectl-v1.19.7-amd64 cni-plugins-linux-amd64-v0.9.0.tgz kubeadm-v1.19.7-amd64 kubelet-v1.19.7-amd64 edgecore-v1.5.0-x86_64
load cloudcore-v1.5.0-x86_64.tar kubespray-v2.15.0-x86_64.tar kubeedge-pause-3.1-x86_64.tar k8s-dns-node-cache-1.16.0-x86_64.tar
cp -rf ${releases_directory}/ /tmp/
run
cp_ ${images_directory}/kubeedge-pause-3.1-x86_64.tar ${CONTAINER_NAME}:/opt/ansible/roles/edgecore/files/
cp_ ${images_directory}/k8s-dns-node-cache-1.16.0-x86_64.tar ${CONTAINER_NAME}:/opt/ansible/roles/edgecore/files/
cp_ ${releases_directory}/edgecore-v1.5.0-x86_64 ${CONTAINER_NAME}:/opt/ansible/roles/edgecore/files/
deploy
