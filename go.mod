module github.com/fabedge/fabedge

go 1.23.0

require (
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/bep/debounce v1.2.0
	github.com/coredns/caddy v1.1.1
	github.com/coredns/coredns v1.8.0
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc
	github.com/fsnotify/fsnotify v1.7.0
	github.com/go-chi/chi/v5 v5.0.0
	github.com/go-logr/logr v1.4.2
	github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/hashicorp/memberlist v0.1.3
	github.com/moby/ipvs v1.1.0
	github.com/olekukonko/tablewriter v0.0.4
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.35.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.19.1
	github.com/spf13/pflag v1.0.5
	github.com/strongswan/govici v0.5.1
	github.com/vishvananda/netlink v1.3.1-0.20250206174618-62fb240731fa
	golang.org/x/sys v0.26.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/klog/v2 v2.130.1
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738
	sigs.k8s.io/controller-runtime v0.9.1
)

replace github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2

require (
	github.com/miekg/dns v1.1.34
	k8s.io/client-go v0.22.5
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.32.7
)

require (
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.18 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.13 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/DataDog/datadog-go v3.5.0+incompatible // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/armon/go-metrics v0.0.0-20180917152333-f0300d1749da // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cyphar/filepath-securejoin v0.3.4 // indirect
	github.com/dnstap/golang-dnstap v0.2.2 // indirect
	github.com/evanphx/json-patch v4.11.0+incompatible // indirect
	github.com/farsightsec/golang-framestream v0.3.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/flynn/go-shlex v0.0.0-20150515145356-3f9db97f8568 // indirect
	github.com/form3tech-oss/jwt-go v3.2.3+incompatible // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/cadvisor v0.51.0 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.0.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.3 // indirect
	github.com/hashicorp/go-multierror v1.0.0 // indirect
	github.com/hashicorp/go-sockaddr v1.0.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/lithammer/dedent v1.1.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-runewidth v0.0.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2 // indirect
	github.com/mistifyio/go-zfs v2.1.2-0.20190413222219-f784269be439+incompatible // indirect
	github.com/moby/spdystream v0.5.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/runc v1.2.1 // indirect
	github.com/opencontainers/runtime-spec v1.2.0 // indirect
	github.com/opentracing-contrib/go-observer v0.0.0-20170622124052-a52f23424492 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/openzipkin-contrib/zipkin-go-opentracing v0.4.5 // indirect
	github.com/openzipkin/zipkin-go v0.2.2 // indirect
	github.com/philhofer/fwd v1.0.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/cobra v1.8.1 // indirect
	github.com/tinylib/msgp v1.1.2 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	go.etcd.io/etcd/api/v3 v3.5.16 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.16 // indirect
	go.etcd.io/etcd/client/v3 v3.5.16 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.53.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0 // indirect
	go.opentelemetry.io/otel v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp v0.20.0 // indirect
	go.opentelemetry.io/otel/metric v1.28.0 // indirect
	go.opentelemetry.io/otel/sdk v1.28.0 // indirect
	go.opentelemetry.io/otel/sdk/export/metric v0.20.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v0.20.0 // indirect
	go.opentelemetry.io/otel/trace v1.28.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/atomic v1.7.0
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/oauth2 v0.23.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/term v0.25.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	golang.org/x/time v0.7.0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/grpc v1.65.0 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
	gopkg.in/DataDog/dd-trace-go.v1 v1.27.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	k8s.io/apiextensions-apiserver v0.21.2 // indirect
	k8s.io/apiserver v0.22.5 // indirect
	k8s.io/cloud-provider v0.22.5 // indirect
	k8s.io/component-base v0.22.5 // indirect
	k8s.io/component-helpers v0.22.5 // indirect
	k8s.io/kube-openapi v0.0.0-20241105132330-32ad38e42d3f // indirect
	k8s.io/kube-proxy v0.0.0 // indirect
	k8s.io/mount-utils v0.22.5 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.31.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.2 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

require (
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	github.com/Microsoft/hnslib v0.1.1 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/emicklei/go-restful v2.16.0+incompatible // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	go.opentelemetry.io/otel/internal/metric v0.27.0 // indirect
	golang.org/x/crypto v0.28.0 // indirect
	golang.org/x/net v0.30.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240826202546-f6391c0de4c7 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	k8s.io/controller-manager v0.22.5 // indirect
	k8s.io/kubelet v0.0.0 // indirect
	sigs.k8s.io/knftables v0.0.17 // indirect
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
