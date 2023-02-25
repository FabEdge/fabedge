module github.com/fabedge/fabedge

go 1.16

require (
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/bep/debounce v1.2.0
	github.com/coredns/caddy v1.1.1
	github.com/coredns/coredns v1.8.0
	github.com/coreos/go-iptables v0.6.0
	github.com/davecgh/go-spew v1.1.1
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-chi/chi/v5 v5.0.0
	github.com/go-logr/logr v0.4.0
	github.com/golang-jwt/jwt/v4 v4.2.0
	github.com/hashicorp/memberlist v0.1.3
	github.com/moby/ipvs v1.0.1
	github.com/olekukonko/tablewriter v0.0.4
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.1 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/strongswan/govici v0.5.1
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/sys v0.1.0
	gopkg.in/yaml.v3 v3.0.0
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/controller-runtime v0.9.1
)

replace github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2

require (
	k8s.io/client-go v0.22.5
	k8s.io/kubernetes v1.22.5
)

replace (
	k8s.io/api => k8s.io/api v0.22.5
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.22.5
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.5
	k8s.io/apiserver => k8s.io/apiserver v0.22.5
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.22.5
	k8s.io/client-go => k8s.io/client-go v0.22.5
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.22.5
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.5
	k8s.io/code-generator => k8s.io/code-generator v0.22.5
	k8s.io/component-base => k8s.io/component-base v0.22.5
	k8s.io/component-helpers => k8s.io/component-helpers v0.22.5
	k8s.io/controller-manager => k8s.io/controller-manager v0.22.5
	k8s.io/cri-api => k8s.io/cri-api v0.22.5
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.22.5
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.22.5
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.22.5
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.22.5
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.22.5
	k8s.io/kubectl => k8s.io/kubectl v0.22.5
	k8s.io/kubelet => k8s.io/kubelet v0.22.5
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.22.5
	k8s.io/metrics => k8s.io/metrics v0.22.5
	k8s.io/mount-utils => k8s.io/mount-utils v0.22.5
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.22.5
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.22.5
)
