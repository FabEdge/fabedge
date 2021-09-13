// Copyright 2021 BoCloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operator

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/jjeffery/stringset"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/operator/allocator"
	apis "github.com/fabedge/fabedge/pkg/operator/apis/community/v1alpha1"
	agentctl "github.com/fabedge/fabedge/pkg/operator/controllers/agent"
	cmmctl "github.com/fabedge/fabedge/pkg/operator/controllers/community"
	connectorctl "github.com/fabedge/fabedge/pkg/operator/controllers/connector"
	proxyctl "github.com/fabedge/fabedge/pkg/operator/controllers/proxy"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
	certutil "github.com/fabedge/fabedge/pkg/util/cert"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
	secretutil "github.com/fabedge/fabedge/pkg/util/secret"
)

var dns1123Reg, _ = regexp.Compile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

type Options struct {
	Namespace        string
	EdgePodCIDR      string
	EndpointIDFormat string
	EdgeLabels       []string
	CNIType          string

	CASecretName string
	Agent        agentctl.Config
	Connector    connectorctl.Config
	Proxy        proxyctl.Config

	ManagerOpts manager.Options

	Store       storepkg.Interface
	NewEndpoint types.NewEndpointFunc
	Manager     manager.Manager
}

func (opts *Options) AddFlags(flag *pflag.FlagSet) {
	flag.StringVar(&opts.Namespace, "namespace", "fabedge", "The namespace in which operator will get or create objects, includes pods, secrets and configmaps")
	flag.StringVar(&opts.CNIType, "cni-type", "", "The CNI name in your kubernetes cluster")
	flag.StringVar(&opts.EdgePodCIDR, "edge-pod-cidr", "", "Specify range of IP addresses for the edge pod. If set, fabedge-operator will automatically allocate CIDRs for every edge node, configure this when you use Calico")
	flag.StringVar(&opts.EndpointIDFormat, "endpoint-id-format", "C=CN, O=fabedge.io, CN={node}", "the id format of tunnel endpoint")
	flag.StringSliceVar(&opts.EdgeLabels, "edge-labels", []string{"node-role.kubernetes.io/edge"}, "Labels to filter edge nodes, (e.g. key1,key2=,key3=value3)")

	flag.StringVar(&opts.Connector.Endpoint.Name, "connector-name", "cloud-connector", "The name of connector, only letters, number and '-' are allowed, the initial must be a letter.")
	flag.StringSliceVar(&opts.Connector.Endpoint.PublicAddresses, "connector-public-addresses", nil, "The connector's public addresses which should be accessible for every edge node, comma separated. Takes single IPv4 addresses, DNS names")
	flag.StringSliceVar(&opts.Connector.ProvidedSubnets, "connector-subnets", nil, "The subnets of connector, mostly the CIDRs to assign pod IP and service ClusterIP")
	flag.StringVar(&opts.Connector.ConfigMapKey.Name, "connector-config", "cloud-tunnels-config", "The name of configmap for connector")
	flag.DurationVar(&opts.Connector.SyncInterval, "connector-config-sync-interval", 5*time.Second, "The interval to synchronize connector configmap")

	flag.StringVar(&opts.Agent.AgentImage, "agent-image", "fabedge/agent:latest", "The image of agent container of agent pod")
	flag.StringVar(&opts.Agent.StrongswanImage, "agent-strongswan-image", "strongswan:5.9.1", "The image of strongswan container of agent pod")
	flag.StringVar(&opts.Agent.ImagePullPolicy, "agent-image-pull-policy", "IfNotPresent", "The imagePullPolicy for all containers of agent pod")
	flag.IntVar(&opts.Agent.AgentLogLevel, "agent-log-level", 3, "The log level of agent")
	flag.BoolVar(&opts.Agent.UseXfrm, "agent-use-xfrm", false, "let agent use xfrm if edge OS supports")
	flag.BoolVar(&opts.Agent.EnableProxy, "agent-enable-proxy", true, "Enable the proxy feature")
	flag.BoolVar(&opts.Agent.MasqOutgoing, "agent-masq-outgoing", true, "Determine if perform outbound NAT from edge pods to outside of the cluster")

	flag.StringVar(&opts.CASecretName, "ca-secret", "fabedge-ca", "The name of secret which contains CA's cert and key")
	flag.StringVar(&opts.Agent.CertOrganization, "cert-organization", certutil.DefaultOrganization, "The organization name for agent's cert")
	flag.Int64Var(&opts.Agent.CertValidPeriod, "cert-validity-period", 365, "The validity period for agent's cert")

	flag.StringVar(&opts.Proxy.IPVSScheduler, "ipvs-scheduler", "rr", "The ipvs scheduler for each service")

	flag.BoolVar(&opts.ManagerOpts.LeaderElection, "leader-election", false, "Determines whether or not to use leader election")
	flag.StringVar(&opts.ManagerOpts.LeaderElectionID, "leader-election-id", "fabedge-operator-leader", "The name of the resource that leader election will use for holding the leader lock")
	opts.ManagerOpts.LeaseDuration = flag.Duration("leader-lease-duration", 15*time.Second, "The duration that non-leader candidates will wait to force acquire leadership")
	opts.ManagerOpts.RenewDeadline = flag.Duration("leader-renew-deadline", 10*time.Second, "The duration that the acting controlplane will retry refreshing leadership before giving up")
	opts.ManagerOpts.RetryPeriod = flag.Duration("leader-retry-period", 2*time.Second, "The duration that the LeaderElector clients should wait between tries of actions")
}

func (opts *Options) Complete() (err error) {
	opts.CNIType = strings.TrimSpace(opts.CNIType)

	parsedEdgeLabels := make(map[string]string)
	for _, label := range opts.EdgeLabels {
		parts := strings.Split(label, "=")
		switch len(parts) {
		case 1:
			parsedEdgeLabels[parts[0]] = ""
		case 2:
			if parts[0] == "" {
				return fmt.Errorf("label's key must not be empty")
			}
			parsedEdgeLabels[parts[0]] = parts[1]
		default:
			return fmt.Errorf("wrong edge label format: %s", strings.Join(parts, "="))
		}
	}
	nodeutil.SetEdgeNodeLabels(parsedEdgeLabels)

	if opts.ShouldAllocatePodCIDR() {
		opts.NewEndpoint = types.GenerateNewEndpointFunc(opts.EndpointIDFormat, nodeutil.GetPodCIDRsFromAnnotation)
		opts.Agent.Allocator, err = allocator.New(opts.EdgePodCIDR)
		if err != nil {
			log.Error(err, "failed to create allocator")
			return err
		}
	} else {
		opts.NewEndpoint = types.GenerateNewEndpointFunc(opts.EndpointIDFormat, nodeutil.GetPodCIDRs)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "failed to load kubeconfig")
		return nil
	}

	kubeClient, err := client.New(cfg, client.Options{})
	if err != nil {
		log.Error(err, "failed to create kube client")
		return err
	}

	opts.ManagerOpts.LeaderElectionNamespace = opts.Namespace
	opts.ManagerOpts.MetricsBindAddress = "0"
	opts.ManagerOpts.Logger = klogr.New().WithName("fabedge-operator")
	opts.Manager, err = manager.New(cfg, opts.ManagerOpts)
	if err != nil {
		log.Error(err, "failed to create controller manager")
		return err
	}

	certManager, err := createCertManager(kubeClient, client.ObjectKey{
		Name:      opts.CASecretName,
		Namespace: opts.Namespace,
	})
	if err != nil {
		log.Error(err, "failed to create cert manager")
		return err
	}

	opts.Store = storepkg.NewStore()

	opts.Agent.Namespace = opts.Namespace
	opts.Agent.CertManager = certManager
	opts.Agent.Manager = opts.Manager
	opts.Agent.Store = opts.Store
	opts.Agent.NewEndpoint = opts.NewEndpoint
	opts.Agent.EnableFlannelMocking = opts.CNIType == constants.CNIFlannel

	opts.Connector.ConfigMapKey.Namespace = opts.Namespace
	opts.Connector.Manager = opts.Manager
	opts.Connector.Store = opts.Store
	opts.Connector.CollectPodCIDRs = !opts.ShouldAllocatePodCIDR()
	opts.Connector.Endpoint.ID = types.GetID(opts.EndpointIDFormat, opts.Connector.Endpoint.Name)

	opts.Proxy.AgentNamespace = opts.Namespace
	opts.Proxy.Manager = opts.Manager
	opts.Proxy.CheckInterval = 5 * time.Second

	return nil
}

func (opts Options) Validate() (err error) {
	if opts.CNIType != constants.CNICalico && opts.CNIType != constants.CNIFlannel {
		return fmt.Errorf("unknown CNI type: %s", opts.CNIType)
	}

	if len(opts.EdgeLabels) == 0 {
		return fmt.Errorf("edge labels is needed")
	}

	if opts.ShouldAllocatePodCIDR() {
		if _, _, err := net.ParseCIDR(opts.EdgePodCIDR); err != nil {
			return err
		}
	}

	if !dns1123Reg.MatchString(opts.Connector.Endpoint.Name) {
		return fmt.Errorf("invalid connector name")
	}

	if !dns1123Reg.MatchString(opts.Connector.ConfigMapKey.Name) {
		return fmt.Errorf("invalid connector config name")
	}

	if len(opts.Connector.Endpoint.PublicAddresses) == 0 {
		return fmt.Errorf("connector public addresses is needed")
	}

	for _, subnet := range opts.Connector.ProvidedSubnets {
		if _, _, err := net.ParseCIDR(subnet); err != nil {
			return err
		}
	}

	policy := corev1.PullPolicy(opts.Agent.ImagePullPolicy)
	if policy != corev1.PullAlways &&
		policy != corev1.PullIfNotPresent &&
		policy != corev1.PullNever {
		return fmt.Errorf("not supported image pull policy: %s", policy)
	}

	// from client-go leaderelection.go
	const JitterFactor = 1.2
	leaseDuration, renewDeadline, retryPeriod := *opts.ManagerOpts.LeaseDuration, *opts.ManagerOpts.RenewDeadline, *opts.ManagerOpts.RetryPeriod
	if leaseDuration <= renewDeadline {
		return fmt.Errorf("leaseDuration must be greater than renewDeadline")
	}
	if renewDeadline <= time.Duration(JitterFactor*float64(retryPeriod)) {
		return fmt.Errorf("renewDeadline must be greater than retryPeriod*JitterFactor")
	}
	if leaseDuration < time.Second {
		return fmt.Errorf("leaseDuration must be greater than 1 second")
	}
	if renewDeadline < time.Second {
		return fmt.Errorf("renewDeadline must be greater than 1 second")
	}
	if retryPeriod < time.Second {
		return fmt.Errorf("retryPeriod must be greater than 1 second")
	}

	return nil
}

func createCertManager(cli client.Client, key client.ObjectKey) (certutil.Manager, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var secret corev1.Secret
	err := cli.Get(ctx, key, &secret)

	if err != nil {
		return nil, err
	}
	certPEM, keyPEM := secretutil.GetCA(secret)

	certDER, err := certutil.DecodePEM(certPEM)
	if err != nil {
		return nil, err
	}

	keyDER, err := certutil.DecodePEM(keyPEM)
	if err != nil {
		return nil, err
	}

	return certutil.NewManger(certDER, keyDER)
}

func (opts Options) RunManager() error {
	if err := opts.Manager.Add(manager.RunnableFunc(opts.initializeControllers)); err != nil {
		log.Error(err, "failed to add init runnable")
		return err
	}

	err := opts.Manager.Start(signals.SetupSignalHandler())
	if err != nil {
		log.Error(err, "failed to start controller manager")
	}

	return err
}

// initializeControllers adds controllers which are related to tunnels management to manager.
// we have to put controller registry logic in a Runnable because allocator and store initialization
// have to be done after leader election is finished, otherwise their data may be out of date
func (opts Options) initializeControllers(ctx context.Context) error {
	err := opts.recordEndpoints(ctx)
	if err != nil {
		log.Error(err, "failed to initialize allocator and store")
		return err
	}

	// todo: ugly!!! try to move getConnectorEndpoint init in Complete
	getConnectorEndpoint, err := connectorctl.AddToManager(opts.Connector)
	if err != nil {
		log.Error(err, "failed to add communities controller to manager")
		return err
	}

	opts.Agent.GetConnectorEndpoint = getConnectorEndpoint
	if err = agentctl.AddToManager(opts.Agent); err != nil {
		log.Error(err, "failed to add agent controller to manager")
		return err
	}

	if err = cmmctl.AddToManager(cmmctl.Config{
		Manager: opts.Manager,
		Store:   opts.Store,
	}); err != nil {
		log.Error(err, "failed to add communities controller to manager")
		return err
	}

	if opts.Agent.EnableProxy {
		if err = proxyctl.AddToManager(opts.Proxy); err != nil {
			log.Error(err, "failed to add proxy controller to manager")
			return err
		}
	}

	return nil
}

func (opts Options) recordEndpoints(ctx context.Context) error {
	cli := opts.Manager.GetClient()
	store := opts.Store

	var nodes corev1.NodeList
	err := cli.List(ctx, &nodes, client.MatchingLabels(nodeutil.GetEdgeNodeLabels()))
	if err != nil {
		return err
	}

	var communities apis.CommunityList
	if err = cli.List(ctx, &communities); err != nil {
		return err
	}
	for _, community := range communities.Items {
		store.SaveCommunity(types.Community{
			Name:    community.Name,
			Members: stringset.New(community.Spec.Members...),
		})
	}

	if !opts.ShouldAllocatePodCIDR() {
		return nil
	}

	for _, node := range nodes.Items {
		ep := opts.NewEndpoint(node)
		if !ep.IsValid() {
			continue
		}

		for _, cidr := range ep.Subnets {
			_, subnet, err := net.ParseCIDR(cidr)
			// todo: maybe we should remove invalid subnet from endpoint here
			if err != nil {
				log.Error(err, "failed to parse subnet of node", "nodeName", node.Name, "node", node)
				continue
			}
			opts.Agent.Allocator.Record(*subnet)
		}

		store.SaveEndpoint(ep)
	}

	return nil
}

func (opts Options) ShouldAllocatePodCIDR() bool {
	return opts.CNIType == constants.CNICalico
}
