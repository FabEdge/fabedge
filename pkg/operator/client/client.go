package client

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/operator/apiserver"
	certutil "github.com/fabedge/fabedge/pkg/util/cert"
)

const defaultTimeout = 5 * time.Second

type Interface interface {
	GetEndpointsAndCommunities() (apiserver.EndpointsAndCommunity, error)
	UpdateEndpoints(endpoints []apis.Endpoint) error
	SignCert(csr []byte) (Certificate, error)
}

type client struct {
	baseURL *url.URL
	client  *http.Client
}

type Certificate struct {
	Raw *x509.Certificate
	DER []byte
	PEM []byte
}

func NewClient(apiServerAddr string, transport http.RoundTripper) (Interface, error) {
	baseURL, err := url.Parse(apiServerAddr)
	if err != nil {
		return nil, err
	}

	return &client{
		baseURL: baseURL,
		client: &http.Client{
			Timeout:   defaultTimeout,
			Transport: transport,
		},
	}, nil
}

func (c *client) SignCert(csr []byte) (cert Certificate, err error) {
	req, err := http.NewRequest(http.MethodPost, join(c.baseURL, apiserver.URLSignCERT), csrBody(csr))
	if err != nil {
		return cert, err
	}
	req.Header.Set("Content-Type", "text/html")

	resp, err := c.client.Do(req)
	if err != nil {
		return cert, err
	}

	return readCertFromResponse(resp)
}

func (c *client) UpdateEndpoints(endpoints []apis.Endpoint) error {
	data, err := json.Marshal(endpoints)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, join(c.baseURL, apiserver.URLUpdateEndpoints), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	_, err = handleResponse(resp)
	return err
}

func (c *client) GetEndpointsAndCommunities() (ea apiserver.EndpointsAndCommunity, err error) {
	req, err := http.NewRequest(http.MethodGet, join(c.baseURL, apiserver.URLGetEndpointsAndCommunities), nil)
	if err != nil {
		return ea, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return ea, err
	}

	data, err := handleResponse(resp)

	err = json.Unmarshal(data, &ea)
	return ea, err
}

func GetCertificate(apiServerAddr string) (cert Certificate, err error) {
	baseURL, err := url.Parse(apiServerAddr)
	if err != nil {
		return cert, err
	}

	cli := &http.Client{
		Timeout: defaultTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := cli.Get(join(baseURL, apiserver.URLGetCA))
	if err != nil {
		return cert, err
	}

	return readCertFromResponse(resp)
}

func SignCertByToken(apiServerAddr string, token string, csr []byte, certPool *x509.CertPool) (cert Certificate, err error) {
	baseURL, err := url.Parse(apiServerAddr)
	if err != nil {
		return cert, err
	}

	cli := &http.Client{
		Timeout: defaultTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		},
	}

	req, err := http.NewRequest(http.MethodPost, join(baseURL, apiserver.URLSignCERT), csrBody(csr))
	if err != nil {
		return cert, err
	}
	req.Header.Set(apiserver.HeaderAuthorization, "bearer "+token)
	req.Header.Set("Content-Type", "text/html")

	resp, err := cli.Do(req)
	if err != nil {
		return cert, err
	}

	return readCertFromResponse(resp)
}

func join(baseURL *url.URL, ref string) string {
	u, _ := baseURL.Parse(ref)
	return u.String()
}

func readCertFromResponse(resp *http.Response) (cert Certificate, err error) {
	cert.PEM, err = handleResponse(resp)
	if err != nil {
		return
	}

	cert.DER, err = certutil.DecodePEM(cert.PEM)
	if err != nil {
		return
	}

	cert.Raw, err = x509.ParseCertificate(cert.DER)

	return cert, err
}

func handleResponse(resp *http.Response) (content []byte, err error) {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		content, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return
		}

		return nil, &HttpError{
			Response: resp,
			Message:  string(content),
		}
	}

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	return ioutil.ReadAll(resp.Body)
}

func csrBody(csr []byte) io.Reader {
	return bytes.NewReader(certutil.EncodeCertRequestPEM(csr))
}
