#!/bin/sh
# Copyright 2021 BoCloud
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


if [ "x$1" = "x" ]; then
   echo "USAGE: genCert.sh nodename"
   exit 1
fi

NODE_NAME=$1
OUTPUT_DIR=/ipsec.d
CA_CERT=${OUTPUT_DIR}/cacerts/ca.pem
CA_KEY=${OUTPUT_DIR}/private/ca_key.pem

ensureDir() {
  [ -d ${OUTPUT_DIR}/private ] || mkdir ${OUTPUT_DIR}/private
  [ -d ${OUTPUT_DIR}/certs ] || mkdir ${OUTPUT_DIR}/certs
  [ -d ${OUTPUT_DIR}/cacerts ] || mkdir ${OUTPUT_DIR}/cacerts
}

ensureCA() {
  if [[ ! -f ${CA_CERT} ]]; then
    ipsec pki --gen --type rsa --size 4096 --outform pem > ${CA_KEY}
    ipsec pki --self --ca --lifetime 3650 --in ${CA_KEY} --type rsa --dn "C=CN, O=StrongSwan, CN=Root CA" --outform pem > ${CA_CERT}
  fi
}

genCert() {
  ipsec pki --gen --type rsa --size 2048 --outform pem > ${OUTPUT_DIR}/private/${1}_key.pem
  ipsec pki --pub --in ${OUTPUT_DIR}/private/${1}_key.pem --type rsa | ipsec pki --issue --lifetime 730 --cacert ${CA_CERT} --cakey ${CA_KEY} --dn "C=CN, O=StrongSwan, CN=$1" --san $1 --flag serverAuth --flag ikeIntermediate --outform pem > ${OUTPUT_DIR}/certs/${1}_cert.pem
}

genSecretsFile() {
  echo ": RSA ${NODE_NAME}_key.pem" >${OUTPUT_DIR}/${NODE_NAME}.ipsec.secrets
}

ensureDir
ensureCA
genCert $1
genSecretsFile
