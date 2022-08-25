package apiserver_test

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/golang-jwt/jwt/v4"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/operator/apiserver"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
	certutil "github.com/fabedge/fabedge/pkg/util/cert"
	timeutil "github.com/fabedge/fabedge/pkg/util/time"
)

var _ = Describe("APIServer", func() {
	var (
		cidrMap                       *types.ClusterCIDRsMap
		store                         storepkg.Interface
		certManager                   certutil.Manager
		clusterName                   string
		clusterCIDRs                  []string
		server                        *http.Server
		rootEndpoint, rootConnector   apis.Endpoint
		childEndpoint, childConnector apis.Endpoint
		community                     apis.Community
		cluster                       apis.Cluster
		privateKey                    *rsa.PrivateKey
	)

	BeforeEach(func() {
		clusterName = "cluster1"
		clusterCIDRs = []string{"192.168.0.0/16"}

		store = storepkg.NewStore()
		cidrMap = types.NewClusterCIDRsMap()
		rootEndpoint = apis.Endpoint{
			ID:              "cluster2.edge1",
			Name:            "cluster2.edge1",
			PublicAddresses: []string{"10.30.1.1"},
			Subnets:         []string{"2.4.0.0/16"},
			NodeSubnets:     []string{"192.168.1.2/32"},
		}
		rootConnector = apis.Endpoint{
			ID:              "cluster2.connector",
			Name:            "cluster2.connector",
			PublicAddresses: []string{"10.40.1.1"},
			Subnets:         []string{"2.5.0.0/16"},
			NodeSubnets:     []string{"192.168.1.3/32"},
		}

		childEndpoint = apis.Endpoint{
			ID:              "cluster1.edge1",
			Name:            "cluster1.edge1",
			PublicAddresses: []string{"10.1.1.1"},
			Subnets:         []string{"2.2.1.1/24"},
			NodeSubnets:     []string{"10.10.1.1/32"},
		}
		childConnector = apis.Endpoint{
			ID:              "cluster1.connector",
			Name:            "cluster1.connector",
			PublicAddresses: []string{"10.1.1.2"},
			Subnets:         []string{"2.2.1.65/24"},
			NodeSubnets:     []string{"10.10.1.2/32"},
		}

		store.SaveEndpoint(rootEndpoint)
		store.SaveEndpoint(rootConnector)
		store.SaveEndpoint(childEndpoint)
		store.SaveEndpoint(childConnector)

		caCertDER, caKeyDER, err := certutil.NewSelfSignedCA(certutil.Config{
			CommonName:     certutil.DefaultCAName,
			Organization:   []string{certutil.DefaultOrganization},
			ValidityPeriod: timeutil.Days(1),
			IsCA:           true,
		})
		Expect(err).Should(BeNil())

		privateKey, err = x509.ParsePKCS1PrivateKey(caKeyDER)
		Expect(err).Should(BeNil())

		certManager, err = certutil.NewManger(caCertDER, caKeyDER, timeutil.Days(1))
		Expect(err).Should(BeNil())

		cluster = apis.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster1",
			},
			Spec: apis.ClusterSpec{
				CIDRs: clusterCIDRs,
				EndPoints: []apis.Endpoint{
					childConnector, childEndpoint,
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), &cluster)).Should(Succeed())

		community = apis.Community{
			ObjectMeta: metav1.ObjectMeta{
				Name: "connectors",
			},
			Spec: apis.CommunitySpec{
				Members: []string{"cluster1.connector", "cluster2.connector"},
			},
		}
		Expect(k8sClient.Create(context.Background(), &community)).Should(Succeed())
		store.SaveCommunity(types.Community{
			Name:    community.Name,
			Members: sets.NewString(community.Spec.Members...),
		})

		server, err = apiserver.New(apiserver.Config{
			Addr:        "localhost:8080",
			CertManager: certManager,
			Client:      k8sClient,
			Store:       store,
			CIDRMap:     cidrMap,
			Log:         klogr.New(),
		})
		Expect(err).Should(BeNil())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), &cluster)).Should(Succeed())
		Expect(k8sClient.Delete(context.Background(), &community)).Should(Succeed())
	})

	Context("With token", func() {
		var clusterToken string

		BeforeEach(func() {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.StandardClaims{
				Subject: clusterName,
			})

			var err error
			clusterToken, err = token.SignedString(privateKey)
			Expect(err).Should(BeNil())
		})

		It("can sign cert for child cluster", func() {
			keyDER, csr, err := certutil.NewCertRequest(certutil.Request{
				CommonName:   "test",
				Organization: []string{"test"},
			})
			Expect(err).Should(BeNil())

			privateKey, _ := x509.ParsePKCS1PrivateKey(keyDER)

			reqBody := bytes.NewBuffer(certutil.EncodeCertRequestPEM(csr))
			req, _ := http.NewRequest("POST", apiserver.URLSignCERT, reqBody)
			req.Header.Add("Authorization", "bearer "+clusterToken)

			resp := executeRequest(req, server)
			Expect(resp.Code).Should(Equal(http.StatusOK))

			content, err := ioutil.ReadAll(resp.Body)
			Expect(err).Should(BeNil())

			certDER, err := certutil.DecodePEM(content)
			Expect(err).Should(BeNil())

			cert, err := x509.ParseCertificate(certDER)
			Expect(err).Should(BeNil())

			Expect(cert.IsCA).Should(BeFalse())
			Expect(cert.Subject.CommonName).Should(Equal("test"))
			Expect(cert.Subject.Organization).Should(ConsistOf("test"))
			Expect(cert.PublicKey).Should(Equal(privateKey.Public()))
			Expect(certManager.VerifyCert(cert, certutil.ExtKeyUsagesServerAndClient)).Should(Succeed())
		})
	})

	Context("With client certificate", func() {
		var connectionState *tls.ConnectionState

		BeforeEach(func() {
			certDER, _, err := certManager.NewCertKey(certutil.Config{
				CommonName:     "client",
				Organization:   []string{certutil.DefaultOrganization},
				ValidityPeriod: time.Hour,
			})
			Expect(err).Should(BeNil())

			cert, err := x509.ParseCertificate(certDER)
			Expect(err).Should(BeNil())

			connectionState = &tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{cert},
			}
		})

		It("can get endpoints and communities needed for a cluster", func() {
			req, _ := http.NewRequest("GET", apiserver.URLGetEndpointsAndCommunities, nil)
			req.TLS = connectionState
			req.Header.Add(apiserver.HeaderClusterName, clusterName)

			resp := executeRequest(req, server)
			Expect(resp.Code).Should(Equal(http.StatusOK))

			content, err := ioutil.ReadAll(resp.Body)
			Expect(err).Should(BeNil())

			var ea apiserver.EndpointsAndCommunity
			Expect(json.Unmarshal(content, &ea)).Should(Succeed())
			Expect(ea.Endpoints).Should(ConsistOf(rootConnector))
			Expect(ea.Communities[community.Name]).Should(ConsistOf(rootConnector.Name, childConnector.Name))
		})

		It("can provide cluster CIDRs", func() {
			clusterName, cidrs := "beijing", []string{"192.168.0.0/18"}
			cidrMap.Set(clusterName, cidrs)

			req, _ := http.NewRequest("GET", apiserver.URLGetCIDRs, nil)
			req.TLS = connectionState
			req.Header.Add(apiserver.HeaderClusterName, clusterName)

			resp := executeRequest(req, server)
			Expect(resp.Code).Should(Equal(http.StatusOK))

			content, err := ioutil.ReadAll(resp.Body)
			Expect(err).Should(BeNil())

			var cidrMap2 map[string][]string
			Expect(json.Unmarshal(content, &cidrMap2)).Should(Succeed())
			Expect(len(cidrMap2)).To(Equal(1))
			Expect(cidrMap2).To(HaveKeyWithValue(clusterName, cidrs))
		})

		It("can update cluster info of requesting cluster", func() {
			requestCluster := apis.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
				Spec: apis.ClusterSpec{
					CIDRs: []string{"2.2.0.0/17"},
					EndPoints: []apis.Endpoint{
						childConnector,
					},
				},
			}

			clusterJson, err := json.Marshal(requestCluster)
			Expect(err).Should(BeNil())

			reqBody := bytes.NewBuffer(clusterJson)
			req, _ := http.NewRequest("PUT", apiserver.URLUpdateCluster, reqBody)
			req.TLS = connectionState
			req.Header.Add(apiserver.HeaderClusterName, clusterName)

			resp := executeRequest(req, server)
			Expect(resp.Code).Should(Equal(http.StatusNoContent))

			err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName}, &cluster)
			Expect(err).Should(BeNil())
			Expect(cluster.Spec.CIDRs).Should(Equal(requestCluster.Spec.CIDRs))
			Expect(cluster.Spec.EndPoints).Should(ConsistOf(childConnector))
		})

		It("can update endpoints of requesting cluster", func() {
			endpoints := []apis.Endpoint{
				childConnector,
			}

			endpointsJson, err := json.Marshal(endpoints)
			Expect(err).Should(BeNil())

			reqBody := bytes.NewBuffer(endpointsJson)
			req, _ := http.NewRequest("PUT", apiserver.URLUpdateEndpoints, reqBody)
			req.TLS = connectionState
			req.Header.Add(apiserver.HeaderClusterName, clusterName)

			resp := executeRequest(req, server)
			Expect(resp.Code).Should(Equal(http.StatusNoContent))

			err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName}, &cluster)
			Expect(err).Should(BeNil())

			Expect(cluster.Spec.CIDRs).Should(Equal(clusterCIDRs))
			Expect(cluster.Spec.EndPoints).Should(ConsistOf(childConnector))
		})

		It("can sign cert for child cluster", func() {
			_, csr, err := certutil.NewCertRequest(certutil.Request{
				CommonName:   "test",
				Organization: []string{"test"},
			})
			Expect(err).Should(BeNil())

			reqBody := bytes.NewBuffer(certutil.EncodeCertRequestPEM(csr))
			req, _ := http.NewRequest("POST", apiserver.URLSignCERT, reqBody)
			req.TLS = connectionState

			resp := executeRequest(req, server)
			Expect(resp.Code).Should(Equal(http.StatusOK))

			content, err := ioutil.ReadAll(resp.Body)
			Expect(err).Should(BeNil())

			certDER, err := certutil.DecodePEM(content)
			Expect(err).Should(BeNil())

			cert, err := x509.ParseCertificate(certDER)
			Expect(err).Should(BeNil())
			Expect(certManager.VerifyCert(cert, certutil.ExtKeyUsagesServerAndClient)).Should(Succeed())
		})
	})

	Context("Without token or client certificate", func() {
		It("response unauthorized for getEndpointsAndCommunities request", func() {
			req, _ := http.NewRequest("GET", apiserver.URLGetEndpointsAndCommunities, nil)
			req.Header.Add(apiserver.HeaderClusterName, clusterName)

			resp := executeRequest(req, server)
			Expect(resp.Code).Should(Equal(http.StatusUnauthorized))
		})

		It("response unauthorized for getEndpointsAndCommunities request", func() {
			req, _ := http.NewRequest("GET", apiserver.URLGetCIDRs, nil)
			req.Header.Add(apiserver.HeaderClusterName, clusterName)

			resp := executeRequest(req, server)
			Expect(resp.Code).Should(Equal(http.StatusUnauthorized))
		})

		It("response unauthorized for updateEndpoints request", func() {
			endpoints := []apis.Endpoint{
				childConnector,
			}

			endpointsJson, err := json.Marshal(endpoints)
			Expect(err).Should(BeNil())

			reqBody := bytes.NewBuffer(endpointsJson)
			req, _ := http.NewRequest("PUT", apiserver.URLUpdateEndpoints, reqBody)
			req.Header.Add(apiserver.HeaderClusterName, clusterName)

			resp := executeRequest(req, server)
			Expect(resp.Code).Should(Equal(http.StatusUnauthorized))
		})

		It("response unauthorized for updateEndpoints request", func() {
			cluster := apis.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
				Spec: apis.ClusterSpec{
					CIDRs: clusterCIDRs,
					EndPoints: []apis.Endpoint{
						childConnector,
					},
				},
			}

			clusterJson, err := json.Marshal(cluster)
			Expect(err).Should(BeNil())

			reqBody := bytes.NewBuffer(clusterJson)
			req, _ := http.NewRequest("PUT", apiserver.URLUpdateCluster, reqBody)
			req.Header.Add(apiserver.HeaderClusterName, clusterName)

			resp := executeRequest(req, server)
			Expect(resp.Code).Should(Equal(http.StatusUnauthorized))
		})

		It("response unauthorized for signCert request", func() {
			_, csr, err := certutil.NewCertRequest(certutil.Request{
				CommonName:   "test",
				Organization: []string{"test"},
			})
			Expect(err).Should(BeNil())

			reqBody := bytes.NewBuffer(certutil.EncodeCertRequestPEM(csr))
			req, _ := http.NewRequest("POST", apiserver.URLSignCERT, reqBody)

			resp := executeRequest(req, server)
			Expect(resp.Code).Should(Equal(http.StatusUnauthorized))
		})
	})
})

func executeRequest(req *http.Request, s *http.Server) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	s.Handler.ServeHTTP(rr, req)

	return rr
}
