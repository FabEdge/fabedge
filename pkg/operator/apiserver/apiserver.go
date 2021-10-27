package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	certutil "github.com/fabedge/fabedge/pkg/util/cert"
)

const (
	URLGetCA                      = "/api/ca-cert"
	URLSignCERT                   = "/api/sign-cert"
	URLUpdateEndpoints            = "/api/endpoints"
	URLGetEndpointsAndCommunities = "/api/endpoints-and-communities"

	keyCluster = "cluster"
)

type Config struct {
	Addr        string
	CertManager certutil.Manager
	Client      client.Client
	Log         logr.Logger
	Store       storepkg.Interface
}

type EndpointsAndCommunity struct {
	Communities map[string][]string `json:"communities,omitempty"`
	Endpoints   []apis.Endpoint     `json:"endpoints,omitempty"`
}

func New(cfg Config) (*http.Server, error) {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Get(URLGetCA, cfg.getCACert)

	r.Group(func(r chi.Router) {
		r.Use(cfg.verifyAuthorization)
		r.Post(URLSignCERT, cfg.signCert)
		r.Put(URLUpdateEndpoints, cfg.updateEndpoints)
		r.Get(URLGetEndpointsAndCommunities, cfg.getEndpointsAndCommunity)
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

	clusterName := r.Context().Value(keyCluster).(string)
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
	clusterName := r.Context().Value(keyCluster).(string)

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

			communitySet[community.Name] = community.Members.Values()
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

func (cfg Config) verifyAuthorization(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("authorization")
		if tokenString == "" {
			cfg.response(w, http.StatusUnauthorized, "Invalid authorization token")
			return
		}

		// tokenString has a prefix "bearer " which is 7 chars long
		token, err := jwt.Parse(tokenString[7:], func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("wrong sign method")
			}

			return cfg.CertManager.GetCACert().PublicKey, nil
		})
		if err != nil {
			cfg.response(w, http.StatusUnauthorized, err.Error())
			return
		}

		if !token.Valid {
			cfg.response(w, http.StatusUnauthorized, "Invalid authorization token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			cfg.response(w, http.StatusBadRequest, "no cluster found in token")
			return
		}

		ctx := context.WithValue(r.Context(), keyCluster, claims[keyCluster])
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (cfg Config) response(w http.ResponseWriter, statusCode int, msg string) {
	w.WriteHeader(statusCode)
	_, err := w.Write([]byte(msg))
	if err != nil {
		cfg.Log.Error(err, "failed to write http response")
	}
}
