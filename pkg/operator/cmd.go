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
	"flag"
	"net"
	"strings"
	"time"

	"github.com/jjeffery/stringset"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/fabedge/fabedge/pkg/common/about"
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
	timeutil "github.com/fabedge/fabedge/pkg/util/time"
)

var log = klogr.New().WithName("agent")

type recordSubnetFunc func(ipNet net.IPNet)

func defaultRecordSubnet(ipNet net.IPNet) {}

func init() {
	_ = apis.AddToScheme(scheme.Scheme)
}

func Execute() error {
	klog.InitFlags(nil)
	defer klog.Flush()
	// init klog level
	_ = flag.Set("v", "3")
	flag.Parse()

	if version {
		about.DisplayVersion()
		return nil
	}

	if err := validateFlags(); err != nil {
		log.Error(err, "invalid arguments found")
		return err
	}

	return startManager()
}

func startManager() error {
	leaseDuration := time.Duration(leaderLeaseDuration) * time.Second
	renewDeadline := time.Duration(leaderRenewDeadline) * time.Second
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{
		MetricsBindAddress: "0",
		Logger:             klogr.New().WithName("agent"),

		LeaderElection:          leaderElection,
		LeaderElectionID:        leaderElectionID,
		LeaderElectionNamespace: namespace,
		LeaseDuration:           &leaseDuration,
		RenewDeadline:           &renewDeadline,
	})
	if err != nil {
		log.Error(err, "failed to create controller manager")
		return err
	}

	if err = mgr.Add(initializeControllers(mgr)); err != nil {
		log.Error(err, "failed to add init runnable")
		return err
	}

	return mgr.Start(signals.SetupSignalHandler())
}

// initializeControllers adds controllers which are related to tunnels management to manager.
// we have to put controller registry logic in a Runnable because allocator and store initialization
// have to be done after leader election is finished, otherwise their data may be out of date
func initializeControllers(mgr manager.Manager) manager.Runnable {
	return manager.RunnableFunc(func(ctx context.Context) error {
		var err error
		var newEndpoint types.NewEndpointFunc
		var alloc allocator.Interface
		var recordSubnet recordSubnetFunc
		if allocatePodCIDR {
			newEndpoint = types.GenerateNewEndpointFunc(endpointIDFormat, nodeutil.GetPodCIDRsFromAnnotation)
			alloc, err = allocator.New(edgePodCIDR)
			if err != nil {
				return err
			}
			recordSubnet = alloc.Record
		} else {
			newEndpoint = types.GenerateNewEndpointFunc(endpointIDFormat, nodeutil.GetPodCIDRs)
			recordSubnet = defaultRecordSubnet
		}

		store, err := initStore(mgr.GetClient(), newEndpoint, recordSubnet)
		if err != nil {
			log.Error(err, "failed to initialize allocator and store")
			return err
		}

		certManager, err := createCertManager(mgr.GetClient())
		if err != nil {
			log.Error(err, "failed to create cert manager")
			return err
		}

		getConnectorEndpoint, err := connectorctl.AddToManager(connectorctl.Config{
			Manager:             mgr,
			Store:               store,
			Namespace:           namespace,
			ConnectorConfigName: connectorConfig,
			ConnectorID:         types.GetID(endpointIDFormat, connectorName),
			ConnectorName:       connectorName,
			ConnectorIP:         connectorIP,
			ProvidedSubnets:     strings.Split(connectorSubnets, ","),
			CollectPodCIDRs:     !allocatePodCIDR,
			Interval:            timeutil.Seconds(connectorConfigSyncInterval),
		})
		if err != nil {
			log.Error(err, "failed to add communities controller to manager")
			return err
		}

		agentConfig := agentctl.Config{
			Manager:              mgr,
			Allocator:            alloc,
			Store:                store,
			NewEndpoint:          newEndpoint,
			GetConnectorEndpoint: getConnectorEndpoint,

			Namespace:       namespace,
			AgentImage:      agentImage,
			StrongswanImage: strongswanImage,
			AllocatePodCIDR: allocatePodCIDR,
			MasqOutgoing:    masqOutgoing,
			UseXfrm:         useXfrm,
			EnableProxy:     enableProxy,

			CertManager:      certManager,
			CertOrganization: certOrganization,
			CertValidPeriod:  certValidPeriod,
		}
		if err = agentctl.AddToManager(agentConfig); err != nil {
			log.Error(err, "failed to add agent controller to manager")
			return err
		}

		if err = cmmctl.AddToManager(cmmctl.Config{
			Manager: mgr,
			Store:   store,
		}); err != nil {
			log.Error(err, "failed to add communities controller to manager")
			return err
		}

		if enableProxy {
			if err = proxyctl.AddToManager(proxyctl.Config{
				Manager:        mgr,
				AgentNamespace: namespace,
				CheckInterval:  10 * time.Second,
				IPVSScheduler:  ipvsScheduler,
			}); err != nil {
				log.Error(err, "failed to add proxy controller to manager")
				return err
			}
		}

		return nil
	})
}

func initStore(cli client.Client, newEndpoint types.NewEndpointFunc, recordSubnet recordSubnetFunc) (storepkg.Interface, error) {
	store := storepkg.NewStore()

	var nodes corev1.NodeList
	err := cli.List(context.Background(), &nodes, client.HasLabels{"node-role.kubernetes.io/edge"})
	if err != nil {
		return nil, err
	}

	var communities apis.CommunityList
	if err = cli.List(context.Background(), &communities); err != nil {
		return nil, err
	}
	for _, community := range communities.Items {
		store.SaveCommunity(types.Community{
			Name:    community.Name,
			Members: stringset.New(community.Spec.Members...),
		})
	}

	for _, node := range nodes.Items {
		ep := newEndpoint(node)
		if ep.IP == "" || len(ep.Subnets) == 0 {
			continue
		}

		for _, cidr := range ep.Subnets {
			_, subnet, err := net.ParseCIDR(cidr)
			// todo: maybe we should remove invalid subnet from endpoint here
			if err != nil {
				log.Error(err, "failed to parse subnet of node", "nodeName", node.Name, "node", node)
				continue
			}
			recordSubnet(*subnet)
		}

		store.SaveEndpoint(ep)
	}

	return store, err
}

func createCertManager(cli client.Client) (certutil.Manager, error) {
	var secret corev1.Secret
	err := cli.Get(context.Background(), client.ObjectKey{
		Name:      caSecretName,
		Namespace: namespace,
	}, &secret)

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
