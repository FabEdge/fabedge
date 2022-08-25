package apiserver

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-logr/logr"
	"github.com/golang-jwt/jwt/v4"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
	certutil "github.com/fabedge/fabedge/pkg/util/cert"
)

const (
	URLGetCA                      = "/api/ca-cert"
	URLSignCERT                   = "/api/sign-cert"
	URLUpdateEndpoints            = "/api/endpoints"
	URLUpdateCluster              = "/api/cluster"
	URLGetEndpointsAndCommunities = "/api/endpoints-and-communities"
	URLGetCIDRs                   = "/api/cidrs"

	HeaderClusterName   = "X-FabEdge-Cluster"
	HeaderAuthorization = "Authorization"
)

type Config struct {
	Addr        string
	PublicKey   *rsa.PublicKey
	CertManager certutil.Manager
	Client      client.Client
	Log         logr.Logger
	Store       storepkg.Interface
	CIDRMap     *types.ClusterCIDRsMap
}

type EndpointsAndCommunity struct {
	Communities map[string][]string `json:"communities,omitempty"`
	Endpoints   []apis.Endpoint     `json:"endpoints,omitempty"`
}

func New(cfg Config) (*http.Server, error) {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get(URLGetCA, cfg.getCACert)
	r.Post(URLSignCERT, cfg.signCert)

	r.Group(func(r chi.Router) {
		r.Use(cfg.verifyCert)
		// /api/endpoints is deprecated and /api/cluster is preferred, we leave it here for backward compatibility
		r.Put(URLUpdateEndpoints, cfg.updateEndpoints)
		r.Put(URLUpdateCluster, cfg.updateCluster)

		r.Get(URLGetEndpointsAndCommunities, cfg.getEndpointsAndCommunity)
		r.Get(URLGetCIDRs, cfg.getCIDRs)
	})

	return &http.Server{
		Addr:    cfg.Addr,
		Handler: r,
	}, nil
}

func (cfg Config) health(w http.ResponseWriter, r *http.Request) {
	w.Write(cfg.CertManager.GetCACertPEM())
}

func (cfg Config) getCACert(w http.ResponseWriter, r *http.Request) {
	w.Write(cfg.CertManager.GetCACertPEM())
}

func (cfg Config) signCert(w http.ResponseWriter, r *http.Request) {
	if r.TLS != nil && len(r.TLS.PeerCertificates) != 0 {
		if err := cfg.CertManager.VerifyCert(r.TLS.PeerCertificates[0], certutil.ExtKeyUsagesServerAndClient); err != nil {
			cfg.Log.Error(err, "client certificate is invalid")
			cfg.response(w, http.StatusUnauthorized, fmt.Sprintf("invalid certificate: %s", err))
			return
		}

		cfg.doSignCert(w, r)
		return
	}

	if err := cfg.verifyAuthorization(r); err != nil {
		cfg.response(w, http.StatusUnauthorized, fmt.Sprintf("invalid token: %s", err))
		return
	}

	cfg.doSignCert(w, r)
}

func (cfg Config) doSignCert(w http.ResponseWriter, r *http.Request) {
	csrPEM, err := ioutil.ReadAll(r.Body)
	if err != nil {
		cfg.response(w, http.StatusBadRequest, fmt.Sprintf("failed to read request body: %s", err))
		return
	}

	csr, err := certutil.DecodePEM(csrPEM)
	if err != nil {
		cfg.response(w, http.StatusBadRequest, err.Error())
		return
	}

	certDER, err := cfg.CertManager.SignCert(csr)
	certPEM := certutil.EncodeCertPEM(certDER)

	w.Write(certPEM)
}

func (cfg Config) verifyAuthorization(r *http.Request) error {
	tokenString := r.Header.Get("authorization")
	if tokenString == "" {
		return fmt.Errorf("invalid authorization token")
	}

	var claims jwt.StandardClaims
	// tokenString has a prefix "bearer " which is 7 chars long
	token, err := jwt.ParseWithClaims(tokenString[7:], &claims, func(token *jwt.Token) (interface{}, error) {
		return cfg.CertManager.GetCACert().PublicKey, nil
	})
	if err != nil {
		return err
	}

	if !token.Valid {
		return fmt.Errorf("invalid authorization token")
	}

	return nil
}

func (cfg Config) verifyCert(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			cfg.response(w, http.StatusUnauthorized, "a client certificate is required")
			return
		}

		if err := cfg.CertManager.VerifyCert(r.TLS.PeerCertificates[0], certutil.ExtKeyUsagesServerAndClient); err != nil {
			cfg.Log.Error(err, "client certificate is invalid")
			cfg.response(w, http.StatusUnauthorized, fmt.Sprintf("invalid certificate: %s", err))
			return
		}

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func (cfg Config) updateCluster(w http.ResponseWriter, r *http.Request) {
	jsonData, err := ioutil.ReadAll(r.Body)
	if err != nil {
		cfg.response(w, http.StatusBadRequest, fmt.Sprintf("failed to read request body: %s", err))
		return
	}

	var reqCluster apis.Cluster
	if err = json.Unmarshal(jsonData, &reqCluster); err != nil {
		cfg.response(w, http.StatusBadRequest, err.Error())
		return
	}

	// It's ok cluster.Spec.CIDRs is empty since it is not necessary
	if len(reqCluster.Spec.EndPoints) == 0 {
		cfg.response(w, http.StatusBadRequest, "at least one endpoint is required")
		return
	}

	clusterName := cfg.getCluster(r)
	var cluster apis.Cluster
	err = cfg.Client.Get(r.Context(), client.ObjectKey{Name: clusterName}, &cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			cfg.response(w, http.StatusNotFound, fmt.Sprintf("unknown cluster %s", clusterName))
			return
		}

		cfg.response(w, http.StatusInternalServerError, err.Error())
		return
	}

	cluster.Spec.CIDRs = reqCluster.Spec.CIDRs
	cluster.Spec.EndPoints = reqCluster.Spec.EndPoints
	if err := cfg.Client.Update(r.Context(), &cluster); err != nil {
		cfg.response(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Write(nil)
}

func (cfg Config) updateEndpoints(w http.ResponseWriter, r *http.Request) {
	endpointsJson, err := ioutil.ReadAll(r.Body)
	if err != nil {
		cfg.response(w, http.StatusBadRequest, fmt.Sprintf("failed to read request body: %s", err))
		return
	}

	var endpoints []apis.Endpoint
	if err = json.Unmarshal(endpointsJson, &endpoints); err != nil {
		cfg.response(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(endpoints) == 0 {
		cfg.response(w, http.StatusBadRequest, "at least one endpoint is required")
		return
	}

	clusterName := cfg.getCluster(r)
	var cluster apis.Cluster
	err = cfg.Client.Get(r.Context(), client.ObjectKey{Name: clusterName}, &cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			cfg.response(w, http.StatusNotFound, fmt.Sprintf("unknown cluster %s", clusterName))
			return
		}

		cfg.response(w, http.StatusInternalServerError, err.Error())
		return
	}

	cluster.Spec.EndPoints = endpoints
	if err := cfg.Client.Update(r.Context(), &cluster); err != nil {
		cfg.response(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
	w.Write(nil)
}

func (cfg Config) getEndpointsAndCommunity(w http.ResponseWriter, r *http.Request) {
	clusterName := cfg.getCluster(r)

	var cluster apis.Cluster
	err := cfg.Client.Get(r.Context(), client.ObjectKey{Name: clusterName}, &cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			cfg.response(w, http.StatusNotFound, fmt.Sprintf("unknown cluster %s", clusterName))
			return
		}

		cfg.response(w, http.StatusInternalServerError, err.Error())
		return
	}

	communitySet := make(map[string][]string)
	endpointNameSet := sets.NewString()
	for _, endpoint := range cluster.Spec.EndPoints {
		communities := cfg.Store.GetCommunitiesByEndpoint(endpoint.Name)

		for _, community := range communities {
			_, ok := communitySet[community.Name]
			if ok {
				continue
			}

			communitySet[community.Name] = community.Members.List()
			for _, name := range communitySet[community.Name] {
				// skip endpoints which are from child cluster
				if !strings.HasPrefix(name, clusterName) {
					endpointNameSet.Insert(name)
				}
			}
		}
	}

	ea := EndpointsAndCommunity{
		Endpoints:   cfg.Store.GetEndpoints(endpointNameSet.List()...),
		Communities: communitySet,
	}

	content, _ := json.Marshal(&ea)

	w.Header().Add("Content-Type", "application/json")
	w.Write(content)
}

func (cfg Config) getCIDRs(w http.ResponseWriter, r *http.Request) {
	cidrMap := cfg.CIDRMap.GetCopy()
	content, _ := json.Marshal(&cidrMap)

	w.Header().Add("Content-Type", "application/json")
	w.Write(content)
}

func (cfg Config) response(w http.ResponseWriter, statusCode int, msg string) {
	w.WriteHeader(statusCode)
	_, err := w.Write([]byte(msg))
	if err != nil {
		cfg.Log.Error(err, "failed to write http response")
	}
}

func (cfg Config) getCluster(r *http.Request) string {
	return r.Header.Get(HeaderClusterName)
}
