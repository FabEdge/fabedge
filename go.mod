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
	github.com/olekukonko/tablewriter v0.0.1
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.1 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/strongswan/govici v0.5.1
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/sys v0.0.0-20210603081109-ebe580a85c40
	gopkg.in/yaml.v3 v3.0.0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/klog/v2 v2.8.0
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/controller-runtime v0.9.1
)

replace github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
