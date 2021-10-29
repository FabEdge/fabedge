package apiserver_test

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"github.com/fabedge/fabedge/pkg/operator/types"
	"github.com/jjeffery/stringset"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dgrijalva/jwt-go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/klogr"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/operator/apiserver"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	certutil "github.com/fabedge/fabedge/pkg/util/cert"
	timeutil "github.com/fabedge/fabedge/pkg/util/time"
)

var _ = Describe("APIServer", func() {
	var (
		store                         storepkg.Interface
		certManager                   certutil.Manager
		clusterName, clusterToken     string
		server                        *http.Server
		rootEndpoint, rootConnector   apis.Endpoint
		childEndpoint, childConnector apis.Endpoint
		community                     apis.Community
		cluster                       apis.Cluster
	)

	BeforeEach(func() {
		clusterName = "cluster1"

		store = storepkg.NewStore()
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

		privateKey, err := x509.ParsePKCS1PrivateKey(caKeyDER)
		Expect(err).Should(BeNil())

		certManager, err = certutil.NewManger(caCertDER, caKeyDER, timeutil.Days(1))
		Expect(err).Should(BeNil())

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"cluster": clusterName,
		})

		clusterToken, err = token.SignedString(privateKey)
		Expect(err).Should(BeNil())

		cluster = apis.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster1",
			},
			Spec: apis.ClusterSpec{
				Token: clusterToken,
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
			Members: stringset.New(community.Spec.Members...),
		})

		server, err = apiserver.New(apiserver.Config{
			Addr:        "localhost:8080",
			CertManager: certManager,
			Client:      k8sClient,
			Store:       store,
			Log:         klogr.New(),
		})
		Expect(err).Should(BeNil())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), &cluster)).Should(Succeed())
		Expect(k8sClient.Delete(context.Background(), &community)).Should(Succeed())
	})

	It("can get endpoints and communities needed for a cluster", func() {
		req, _ := http.NewRequest("GET", apiserver.URLGetEndpointsAndCommunities, nil)
		req.Header.Add("Authorization", "bearer "+clusterToken)

		resp := executeRequest(req, server)
		Expect(resp.Code).Should(Equal(http.StatusOK))

		content, err := ioutil.ReadAll(resp.Body)
		Expect(err).Should(BeNil())

		var ea apiserver.EndpointsAndCommunity
		Expect(json.Unmarshal(content, &ea)).Should(Succeed())
		Expect(ea.Endpoints).Should(ConsistOf(rootConnector))
		Expect(ea.Communities[community.Name]).Should(ConsistOf(rootConnector.Name, childConnector.Name))
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

	Context("API update endpoints", func() {
		It("can update endpoints of requesting cluster", func() {
			endpoints := []apis.Endpoint{
				childConnector,
			}

			endpointsJson, err := json.Marshal(endpoints)
			Expect(err).Should(BeNil())

			reqBody := bytes.NewBuffer(endpointsJson)
			req, _ := http.NewRequest("PUT", apiserver.URLUpdateEndpoints, reqBody)
			req.Header.Add("Authorization", "bearer "+clusterToken)

			resp := executeRequest(req, server)
			Expect(resp.Code).Should(Equal(http.StatusNoContent))

			err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName}, &cluster)
			Expect(err).Should(BeNil())

			Expect(cluster.Spec.EndPoints).Should(ConsistOf(childConnector))
		})
	})
})

func executeRequest(req *http.Request, s *http.Server) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	s.Handler.ServeHTTP(rr, req)

	return rr
}
