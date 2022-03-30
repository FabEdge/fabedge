module github.com/fabedge/fabedge

go 1.16

require (
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/bep/debounce v1.2.0
	github.com/coreos/go-iptables v0.6.0
	github.com/davecgh/go-spew v1.1.1
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-chi/chi/v5 v5.0.0
	github.com/go-logr/logr v0.3.0
	github.com/golang-jwt/jwt/v4 v4.2.0
	github.com/hashicorp/memberlist v0.1.3
	github.com/miekg/dns v1.1.25 // indirect
	github.com/moby/ipvs v1.0.1
	github.com/olekukonko/tablewriter v0.0.1
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/strongswan/govici v0.5.1
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/sys v0.0.0-20210511113859-b0526f3d8744
	golang.org/x/text v0.3.6 // indirect
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/klog/v2 v2.4.0
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/controller-runtime v0.8.3
)

replace github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
