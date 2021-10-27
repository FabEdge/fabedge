// Copyright 2021 FabEdge Team
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

package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"net"
	"os"
	"time"

	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
)

const (
	DefaultCountry      = "CN"
	DefaultOrganization = "fabedge.io"
	DefaultCAName       = "Fabedge CA"
)

var (
	ExtKeyUsagesServerAndClient = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	ExtKeyUsagesServerOnly      = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	ExtKeyUsagesClientOnly      = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
)

type Config struct {
	CommonName   string
	Organization []string
	Usages       []x509.ExtKeyUsage

	DNSNames []string
	IPs      []net.IP

	ValidityPeriod time.Duration
	IsCA           bool
}

type Request struct {
	CommonName   string
	Organization []string
	DNSNames     []string
	IPs          []net.IP
}

// NewSelfSignedCA create a CA cert/key pair
func NewSelfSignedCA(cfg Config) ([]byte, []byte, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}

	template, err := buildCertTemplate(cfg)
	if err != nil {
		return nil, nil, err
	}

	// creates a CA certificate
	caDER, err := x509.CreateCertificate(rand.Reader, template, template, caKey.Public(), caKey)
	if err != nil {
		return nil, nil, err
	}

	caKeyDER := x509.MarshalPKCS1PrivateKey(caKey)
	return caDER, caKeyDER, nil
}

// NewCertFromCA2 creates certificate and key from specified CA cert/key pair
func NewCertFromCA2(ca, caKey []byte, cfg Config) ([]byte, []byte, error) {
	caCert, err := x509.ParseCertificate(ca)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse a caCert from the given ASN.1 DER data, err: %v", err)
	}
	caKeyRSA, err := x509.ParsePKCS1PrivateKey(caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parses an RSA private key in PKCS #1, ASN.1 DER form, err: %v", err)
	}

	return NewCertFromCA(caCert, caKeyRSA, cfg)
}

// NewCertFromCA creates certificate and key from specified CA cert/key pair
func NewCertFromCA(caCert *x509.Certificate, caKey *rsa.PrivateKey, cfg Config) ([]byte, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate a privateKey, err: %v", err)
	}
	privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey)

	certTemplate, err := buildCertTemplate(cfg)
	if err != nil {
		return nil, nil, err
	}

	certDER, err := x509.CreateCertificate(rand.Reader, certTemplate, caCert, privateKey.Public(), caKey)
	if err != nil {
		return nil, nil, err
	}

	return certDER, privateKeyDER, nil
}

func NewCertRequest(req Request) ([]byte, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   req.CommonName,
			Country:      []string{DefaultCountry},
			Organization: req.Organization,
		},
		IPAddresses: req.IPs,
		DNSNames:    req.DNSNames,
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, template, privateKey)
	if err != nil {
		return nil, nil, err
	}

	keyDER := x509.MarshalPKCS1PrivateKey(privateKey)
	return keyDER, csr, nil
}

func buildCertTemplate(cfg Config) (*x509.Certificate, error) {
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	if len(cfg.CommonName) == 0 {
		return nil, errors.New("must specify a CommonName")
	}

	keyUsage := x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	if cfg.IsCA {
		keyUsage |= x509.KeyUsageCertSign
	}

	template := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Country:      []string{"CN"},
			Organization: cfg.Organization,
		},
		DNSNames:     cfg.DNSNames,
		IPAddresses:  cfg.IPs,
		SerialNumber: serialNumber,
		NotBefore:    time.Now().UTC(),
		NotAfter:     time.Now().Add(cfg.ValidityPeriod),
		KeyUsage:     keyUsage,
		ExtKeyUsage:  cfg.Usages,

		BasicConstraintsValid: cfg.IsCA,
		IsCA:                  cfg.IsCA,

		PublicKeyAlgorithm: x509.RSA,
		SignatureAlgorithm: x509.SHA384WithRSA,
	}
	return &template, nil
}

// VerifyCert verifies the certificate by CA certificate
func VerifyCert(caDER, certDER []byte, usages []x509.ExtKeyUsage) error {
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: certutil.CertificateBlockType, Bytes: caDER}))
	if !ok {
		return fmt.Errorf("failed to parse ca certificate")
	}
	opts := x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: usages,
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return fmt.Errorf("not able to parse certificate: %w", err)
	}

	_, err = cert.Verify(opts)
	return err
}

func SaveCertKeyToFile(certDER []byte, keyDER []byte, certPath, keyPath string) error {
	if err := SaveCertToFile(certDER, certPath); err != nil {
		return fmt.Errorf("failed to save cert to file %s, err: %v", certPath, err)
	}

	return SavePrivateKeyToFile(keyDER, keyPath)
}

func SaveCertToFile(certDER []byte, certPath string) error {
	certPEM := EncodeCertPEM(certDER)
	if certPEM == nil {
		return fmt.Errorf("failed to encoded the certs. certDER has invalid headers")
	}
	return certutil.WriteCert(certPath, certPEM)
}

func SavePrivateKeyToFile(keyDER []byte, keyPath string) error {
	keyPEM := EncodePrivateKeyPEM(keyDER)
	if keyPEM == nil {
		return fmt.Errorf("failed to encoded the private key. keyDER has invalid headers")
	}

	return keyutil.WriteKey(keyPath, keyPEM)
}

func ReadPEMFileAndDecode(filename string) ([]byte, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return DecodePEM(data)
}

func DecodePEM(data []byte) ([]byte, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode pem data")
	}

	return block.Bytes, nil
}

func SaveFile(content []byte, filename string) error {
	return ioutil.WriteFile(filename, content, os.FileMode(0644))
}

func EncodeCertPEM(certDER []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: certutil.CertificateBlockType, Bytes: certDER})
}

func EncodePrivateKeyPEM(privateKeyDER []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: privateKeyDER})
}

func EncodeCertRequestPEM(crs []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: certutil.CertificateRequestBlockType, Bytes: crs})
}
