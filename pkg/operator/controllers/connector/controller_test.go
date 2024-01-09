// Copyright 2021 FabEdge Team
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package connector

import (
	"context"
	"crypto/x509"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/common/netconf"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
	certutil "github.com/fabedge/fabedge/pkg/util/cert"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
	secretutil "github.com/fabedge/fabedge/pkg/util/secret"
	testutil "github.com/fabedge/fabedge/pkg/util/test"
	timeutil "github.com/fabedge/fabedge/pkg/util/time"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Controller", func() {
	var (
		store           storepkg.Interface
		ctx             context.Context
		cancel          context.CancelFunc
		namespace       string
		interval        time.Duration
		config          Config
		node1, edge1    corev1.Node
		connectorPod    corev1.Pod
		connectorLabels = map[string]string{
			"app": "fabedge-connector",
		}

		getConnectorEndpoint types.EndpointGetter
		certManager          certutil.Manager
		getConnectName       = testutil.GenerateGetNameFunc("connector")
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		namespace = "default"
		interval = 2 * time.Second

		store = storepkg.NewStore()

		connectorPod = createConnectorPod(getConnectName(), namespace, connectorLabels)

		node1 = newNormalNode("192.168.1.2", "10.10.10.64/26")
		edge1 = newEdgeNode("10.20.40.183", "2.2.0.65/26")
		Expect(k8sClient.Create(context.Background(), &node1)).To(Succeed())
		Expect(k8sClient.Create(context.Background(), &edge1)).To(Succeed())

		caCertDER, caKeyDER, _ := certutil.NewSelfSignedCA(certutil.Config{
			CommonName:     certutil.DefaultCAName,
			Organization:   []string{certutil.DefaultOrganization},
			IsCA:           true,
			ValidityPeriod: timeutil.Days(365),
		})
		certManager, _ = certutil.NewManger(caCertDER, caKeyDER, timeutil.Days(365))

		mgr, err := manager.New(cfg, manager.Options{
			MetricsBindAddress:     "0",
			HealthProbeBindAddress: "0",
		})
		Expect(err).ShouldNot(HaveOccurred())
		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(ctx)).NotTo(HaveOccurred())
		}()

		mgr.GetCache().WaitForCacheSync(ctx)

		config = Config{
			Manager:          mgr,
			Store:            store,
			CertManager:      certManager,
			CertOrganization: certutil.DefaultOrganization,
			Endpoint: apis.Endpoint{
				ID:              "cloud-connector",
				Name:            "cloud-connector",
				PublicAddresses: []string{"192.168.1.1"},
			},
			GetPodCIDRs: nodeutil.GetPodCIDRs,

			ProvidedSubnets: []string{"10.10.10.1/26"},
			Namespace:       namespace,
			SyncInterval:    interval,

			ConnectorLabels: connectorLabels,
		}
		getConnectorEndpoint, err = AddToManager(config)
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		cancel()
	})

	It("build connector endpoint when node events come", func() {
		cep, ok := store.GetEndpoint(config.Endpoint.Name)
		Expect(ok).Should(BeTrue())

		Expect(cep.PublicAddresses).Should(Equal(config.Endpoint.PublicAddresses))
		Expect(cep.ID).Should(Equal(config.Endpoint.ID))
		Expect(cep.Name).Should(Equal(config.Endpoint.Name))
		Expect(cep.Subnets).Should(ConsistOf(config.ProvidedSubnets[0], nodeutil.GetPodCIDRs(node1)[0]))
		Expect(cep.NodeSubnets).Should(Equal(nodeutil.GetInternalIPs(node1)))
		Expect(cep.Type).Should(Equal(apis.Connector))

		By("create node2 and node3")
		node2 := newNormalNode("192.168.1.3", "10.10.10.128/26")
		node3 := newNormalNode("192.168.1.4", "10.10.10.200/26")
		Expect(k8sClient.Create(context.Background(), &node2)).To(Succeed())
		Expect(k8sClient.Create(context.Background(), &node3)).To(Succeed())
		time.Sleep(time.Second)

		cep, ok = store.GetEndpoint(config.Endpoint.Name)
		Expect(ok).Should(BeTrue())
		Expect(cep.PublicAddresses).Should(Equal(config.Endpoint.PublicAddresses))
		Expect(cep.ID).Should(Equal(config.Endpoint.ID))
		Expect(cep.Name).Should(Equal(config.Endpoint.Name))
		Expect(cep.Subnets).Should(ConsistOf(config.ProvidedSubnets[0], nodeutil.GetPodCIDRs(node1)[0], nodeutil.GetPodCIDRs(node2)[0], nodeutil.GetPodCIDRs(node3)[0]))
		Expect(cep.NodeSubnets).Should(ConsistOf(nodeutil.GetInternalIPs(node1)[0], nodeutil.GetInternalIPs(node2)[0], nodeutil.GetInternalIPs(node3)[0]))

		By("deleting node2")
		Expect(k8sClient.Delete(context.Background(), &node2)).To(Succeed())
		time.Sleep(time.Second)

		cep, ok = store.GetEndpoint(config.Endpoint.Name)
		Expect(ok).Should(BeTrue())
		Expect(cep.Subnets).ShouldNot(ContainElements(nodeutil.GetPodCIDRs(node2)[0]))
		Expect(cep.NodeSubnets).ShouldNot(ContainElements(nodeutil.GetInternalIPs(node2)[0]))

		By("changing node3 to edge node")
		node3.Labels = edgeLabels
		Expect(k8sClient.Update(context.Background(), &node3)).To(Succeed())
		time.Sleep(time.Second)

		cep, ok = store.GetEndpoint(config.Endpoint.Name)
		Expect(ok).Should(BeTrue())
		Expect(cep.Subnets).ShouldNot(ContainElements(nodeutil.GetPodCIDRs(node3)[0]))
		Expect(cep.NodeSubnets).ShouldNot(ContainElements(nodeutil.GetInternalIPs(node3)[0]))
	})

	It("should synchronize connector configmap according to endpoints in store", func() {
		localEdge1 := apis.Endpoint{
			ID:              "edge1",
			Name:            "edge1",
			PublicAddresses: []string{"10.20.40.181"},
			Subnets:         []string{"2.2.0.1/26"},
			NodeSubnets:     []string{"10.20.40.181"},
		}
		localEdge2 := apis.Endpoint{
			ID:              "edge2",
			Name:            "edge2",
			PublicAddresses: []string{"10.20.40.182"},
			Subnets:         []string{"2.2.0.65/26"},
			NodeSubnets:     []string{"10.20.40.182"},
		}
		alienConnector := apis.Endpoint{
			ID:              "alien.connector",
			Name:            "alien.connector",
			PublicAddresses: []string{"10.30.40.182"},
			Subnets:         []string{"2.3.0.65/26"},
			NodeSubnets:     []string{"10.30.40.182"},
		}

		store.SaveEndpointAsLocal(localEdge1)
		store.SaveEndpointAsLocal(localEdge2)
		store.SaveEndpoint(alienConnector)
		store.SaveCommunity(types.Community{
			Name:    "connectors",
			Members: sets.NewString(alienConnector.Name, config.Endpoint.Name),
		})

		time.Sleep(interval + time.Second)

		key := client.ObjectKey{
			Name:      constants.ConnectorConfigName,
			Namespace: config.Namespace,
		}
		var cm corev1.ConfigMap
		Expect(k8sClient.Get(context.Background(), key, &cm)).ShouldNot(HaveOccurred())

		getNetworkConf := func() netconf.NetworkConf {
			conf := netconf.NetworkConf{}
			err := yaml.Unmarshal([]byte(cm.Data[constants.ConnectorConfigFileName]), &conf)
			Expect(err).ShouldNot(HaveOccurred())

			return conf
		}

		conf := getNetworkConf()
		cep := getConnectorEndpoint()
		Expect(conf.Endpoint).To(Equal(cep))
		Expect(conf.Peers).To(ConsistOf(alienConnector, localEdge1, localEdge2))

		By("remove edge2 endpoint")
		store.DeleteEndpoint(localEdge2.Name)

		time.Sleep(2 * interval)

		cm = corev1.ConfigMap{}
		Expect(k8sClient.Get(context.Background(), key, &cm)).ShouldNot(HaveOccurred())

		conf = getNetworkConf()
		Expect(conf.Endpoint).To(Equal(getConnectorEndpoint()))
		Expect(conf.Peers).To(ConsistOf(alienConnector, localEdge1))

		By("Add mediator endpoint")
		store.SaveEndpoint(apis.Endpoint{
			Name:            constants.DefaultMediatorName,
			ID:              config.Endpoint.ID,
			PublicAddresses: config.Endpoint.PublicAddresses,
		})

		time.Sleep(2 * interval)

		cm = corev1.ConfigMap{}
		Expect(k8sClient.Get(context.Background(), key, &cm)).ShouldNot(HaveOccurred())

		conf = getNetworkConf()
		Expect(conf.Mediator).NotTo(BeNil())
	})

	It("should create a tls secret for connector", func() {
		time.Sleep(interval + time.Second)

		key := client.ObjectKey{
			Name:      constants.ConnectorTLSName,
			Namespace: config.Namespace,
		}
		var secret corev1.Secret
		Expect(k8sClient.Get(context.Background(), key, &secret)).Should(Succeed())

		By("Checking TLS secret")
		caCertPEM, certPEM := secretutil.GetCACert(secret), secretutil.GetCert(secret)
		Expect(certManager.VerifyCertInPEM(certPEM, certutil.ExtKeyUsagesServerAndClient)).Should(Succeed())
		Expect(caCertPEM).Should(Equal(certManager.GetCACertPEM()))

		certDER, err := certutil.DecodePEM(certPEM)
		Expect(err).Should(BeNil())

		cert, err := x509.ParseCertificate(certDER)
		Expect(err).Should(BeNil())
		Expect(cert.Subject.Organization[0]).To(Equal(config.CertOrganization))
		Expect(cert.Subject.CommonName).To(Equal(getConnectorEndpoint().Name))

		By("Changing TLS secret with expired cert")
		certDER, keyDER, _ := certManager.NewCertKey(certutil.Config{
			CommonName:     getConnectorEndpoint().Name,
			ValidityPeriod: time.Second,
		})
		secret.Data[corev1.TLSCertKey] = certutil.EncodeCertPEM(certDER)
		secret.Data[corev1.TLSPrivateKeyKey] = certutil.EncodePrivateKeyPEM(keyDER)
		Expect(k8sClient.Update(context.Background(), &secret)).Should(Succeed())

		time.Sleep(2 * interval)

		By("Checking if TLS secret updated")
		secret = corev1.Secret{}
		Expect(k8sClient.Get(context.Background(), key, &secret)).Should(Succeed())
		caCertPEM, certPEM = secretutil.GetCACert(secret), secretutil.GetCert(secret)
		Expect(certManager.VerifyCertInPEM(certPEM, certutil.ExtKeyUsagesServerAndClient)).Should(Succeed())
		Expect(caCertPEM).Should(Equal(certManager.GetCACertPEM()))
	})

	It("should recreate a tls secret for connector if commonName is wrong", func() {
		time.Sleep(interval + time.Second)

		key := client.ObjectKey{
			Name:      constants.ConnectorTLSName,
			Namespace: config.Namespace,
		}
		var secret corev1.Secret
		Expect(k8sClient.Get(context.Background(), key, &secret)).Should(Succeed())

		By("Changing TLS secret with wrong commonName")
		certDER, keyDER, _ := certManager.NewCertKey(certutil.Config{
			CommonName:     "wrong-connector-name",
			ValidityPeriod: time.Hour,
		})
		secret.Data[corev1.TLSCertKey] = certutil.EncodeCertPEM(certDER)
		secret.Data[corev1.TLSPrivateKeyKey] = certutil.EncodePrivateKeyPEM(keyDER)
		Expect(k8sClient.Update(context.Background(), &secret)).Should(Succeed())

		time.Sleep(2 * interval)

		By("Checking if TLS secret updated")
		secret = corev1.Secret{}
		Expect(k8sClient.Get(context.Background(), key, &secret)).Should(Succeed())

		cert, err := parseCertFromSecret(secret)
		Expect(err).Should(BeNil())
		Expect(cert.Subject.CommonName).To(Equal(getConnectorEndpoint().Name))
	})

	It("should delete connector pods if a tls secret is generated", func() {
		expectConnectorDeleted(connectorPod, interval+time.Second)

		By("Create a new connector Pod")
		connectorPod = createConnectorPod(getConnectName(), namespace, connectorLabels)

		By("Changing TLS secret with expired cert")
		key := client.ObjectKey{
			Name:      constants.ConnectorTLSName,
			Namespace: config.Namespace,
		}
		var secret corev1.Secret
		Expect(k8sClient.Get(context.Background(), key, &secret)).Should(Succeed())

		certDER, keyDER, _ := certManager.NewCertKey(certutil.Config{
			CommonName:     getConnectorEndpoint().Name,
			ValidityPeriod: time.Second,
		})
		secret.Data[corev1.TLSCertKey] = certutil.EncodeCertPEM(certDER)
		secret.Data[corev1.TLSPrivateKeyKey] = certutil.EncodePrivateKeyPEM(keyDER)
		Expect(k8sClient.Update(context.Background(), &secret)).Should(Succeed())

		expectConnectorDeleted(connectorPod, 2*interval)
	})
})

func newNormalNode(ip, subnets string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: getNodeName(),
		},
		Spec: corev1.NodeSpec{
			PodCIDR:  subnets,
			PodCIDRs: strings.Split(subnets, ","),
		},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: ip,
				},
			},
		},
	}
}

func newEdgeNode(ip, subnets string) corev1.Node {
	node := newNormalNode(ip, subnets)
	node.Name = getEdgeName()
	node.Labels = edgeLabels

	return node
}

func createConnectorPod(name, namespace string, labels map[string]string) corev1.Pod {
	connector := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
		},
	}
	Expect(k8sClient.Create(context.Background(), &connector)).To(Succeed())

	return connector
}

func expectConnectorDeleted(pod corev1.Pod, timeout time.Duration) {
	key := client.ObjectKey{
		Name:      pod.Name,
		Namespace: pod.Namespace,
	}

	EventuallyWithOffset(1, func() bool {
		err := k8sClient.Get(context.Background(), key, &pod)
		if err == nil {
			return pod.DeletionTimestamp != nil
		} else {
			return errors.IsNotFound(err)
		}
	}, timeout).Should(BeTrue())
}
