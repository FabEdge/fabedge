OUTPUT_DIR := _output
BINARIES := agent connector operator cloud-agent node
IMAGES := $(addsuffix -image, ${BINARIES})

VERSION := $(shell git describe --tags)
STRONGSWAN_VERSION = 5.9.1

BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S%z')
GIT_COMMIT := $(shell git rev-parse --short HEAD)
META := github.com/fabedge/fabedge/pkg/common/about
FLAG_VERSION := ${META}.version=${VERSION}
FLAG_BUILD_TIME := ${META}.buildTime=${BUILD_TIME}
FLAG_GIT_COMMIT := ${META}.gitCommit=${GIT_COMMIT}

GOLDFLAGS ?= -s -w
LDFLAGS := -ldflags "${GOLDFLAGS} -X ${FLAG_VERSION} -X ${FLAG_BUILD_TIME} -X ${FLAG_GIT_COMMIT}"

CRD_OPTIONS ?= "crd"
K8S_VERSION=1.21.2
GOOS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH ?= amd64
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

export KUBEBUILDER_ASSETS ?= $(GOBIN)
export ACK_GINKGO_DEPRECATIONS ?= 1.16.4

all: clean bin

define HELP_INFO
# Build
#
# Args:
#   GOLDFLAGS: Specify GOLDFLAGS to pass options to go build, when GOLDFLAGS is unspecified,
#   it defaults to "-s -w" which strips debug information

#   make all
#   make agent
#   make connector
#   make operator
#   make connector-image
#   make strongswan-image
#   make operator-image
#   make agent-image
#   make e2e-test
endef
help:
	echo ${HELP_INFO}

fmt:
	GOOS=linux go fmt ./...

vet:
	GOOS=linux go vet ./...

bin: fmt vet ${BINARIES}

${BINARIES}: $(if $(QUICK),,fmt vet)
	GOOS=linux go build ${LDFLAGS} -o ${OUTPUT_DIR}/fabedge-$@ ./cmd/$@

.PHONY: test
test:
ifneq (,$(shell which ginkgo))
	ginkgo ./pkg/...
else
	go test ./pkg/...
endif

e2e-test:
	go test ${LDFLAGS} -c ./test/e2e -o ${OUTPUT_DIR}/fabedge-e2e.test

buildx-install:
	docker buildx install > /dev/null 2>&1 || true

${IMAGES}: APP=$(subst -image,,$@)
${IMAGES}: buildx-install
	docker build -t fabedge/${APP}:${VERSION} $(if $(PLATFORM),--platform $(PLATFORM)) $(if $(PUSH),--push) -f build/${APP}/Dockerfile .

fabedge-images: ${IMAGES}

strongswan-image: buildx-install
	docker build -t fabedge/strongswan:${STRONGSWAN_VERSION} $(if $(PLATFORM),--platform $(PLATFORM)) $(if $(PUSH),--push) -f build/strongswan/Dockerfile .

clean:
	go clean -cache -testcache
	rm -rf ${OUTPUT_DIR}

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=fabedge-admin paths="./pkg/..." output:dir:crd=deploy/crds
	@# 因为k8s的bug, 导致必须手动删除一些信息，详细内容参考 https://github.com/kubernetes/kubernetes/issues/91395
#    sed -i '/- protocol/d' build/crds/edge.bocloud.com_edgeapplications.yaml

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object paths="./pkg/..."

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# https://book.kubebuilder.io/reference/envtest.html
install-test-tools:
	curl -sL "https://go.kubebuilder.io/test-tools/${K8S_VERSION}/${GOOS}/${GOARCH}" | \
                    tar -zx -C ${GOBIN} --strip-components=2
