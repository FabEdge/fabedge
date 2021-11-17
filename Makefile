OUTPUT_DIR := _output
BINARIES := agent connector operator cert cloud-agent
BINARIES_QUICK := $(addsuffix -quick, ${BINARIES})
IMAGES := $(addsuffix -image, ${BINARIES})

VERSION := v0.4.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S%z')
GIT_COMMIT := $(shell git rev-parse --short HEAD)
META := github.com/fabedge/fabedge/pkg/common/about
FLAG_VERSION := ${META}.version=${VERSION}
FLAG_BUILD_TIME := ${META}.buildTime=${BUILD_TIME}
FLAG_GIT_COMMIT := ${META}.gitCommit=${GIT_COMMIT}

GOLDFLAGS ?= -s -w
LDFLAGS := -ldflags "${GOLDFLAGS} -X ${FLAG_VERSION} -X ${FLAG_BUILD_TIME} -X ${FLAG_GIT_COMMIT}"

CRD_OPTIONS ?= "crd:trivialVersions=true"
KUBEBUILDER_VERSION ?= 2.3.1
GOOS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH ?= amd64
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

export KUBEBUILDER_ASSETS ?= $(GOBIN)

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

${BINARIES}: fmt vet
	GOOS=linux go build ${LDFLAGS} -o ${OUTPUT_DIR}/fabedge-$@ ./cmd/$@


${BINARIES_QUICK}: APP=$(subst -quick,,$@)
${BINARIES_QUICK}:
	GOOS=linux go build ${LDFLAGS} -o ${OUTPUT_DIR}/fabedge-${APP} ./cmd/${APP}

.PHONY: test
test:
ifneq (,$(shell which ginkgo))
	ginkgo ./pkg/...
else
	go test ./pkg/...
endif

e2e-test:
	go test ${LDFLAGS} -c ./test/e2e -o ${OUTPUT_DIR}/fabedge-e2e.test

${IMAGES}: APP=$(subst -image,,$@)
${IMAGES}:
	docker build -t fabedge/${APP}:latest -f build/${APP}/Dockerfile .

fabedge-images: ${IMAGES}

strongswan-image:
	docker build -t fabedge/strongswan:latest -f build/strongswan/Dockerfile .

installer-image:
	docker build -t fabedge/installer:latest -f build/installer/Dockerfile .

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
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

install-test-dependencies:
	curl -sL https://github.com/kubernetes-sigs/kubebuilder/releases/download/v$(KUBEBUILDER_VERSION)/kubebuilder_$(KUBEBUILDER_VERSION)_$(GOOS)_$(GOARCH).tar.gz | \
                    tar -zx -C ${GOBIN} --strip-components=2
