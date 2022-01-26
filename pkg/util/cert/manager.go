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
	"encoding/pem"
	"fmt"
	"time"
)

type Manager interface {
	// NewCertKey Create a cert/key pair from CA with specified config
	NewCertKey(cfg Config) (certDER []byte, keyDER []byte, err error)
	SignCert(csr []byte) ([]byte, error)
	VerifyCert(cert *x509.Certificate, usages []x509.ExtKeyUsage) error
	VerifyCertInPEM(certPEM []byte, usages []x509.ExtKeyUsage) error
	GetCACert() *x509.Certificate
	GetCACertPEM() []byte
}

type manager struct {
	caCertPEM   []byte
	caCert      *x509.Certificate
	caKey       *rsa.PrivateKey
	certPool    *x509.CertPool
	validPeriod time.Duration
}

func NewManger(caDER, caKeyDER []byte, validPeriod time.Duration) (Manager, error) {
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse a caCert from the given ASN.1 DER data, err: %v", err)
	}

	caKey, err := x509.ParsePKCS1PrivateKey(caKeyDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parses an RSA private key in PKCS #1, ASN.1 DER form, err: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	return &manager{
		caCertPEM:   EncodeCertPEM(caDER),
		caCert:      caCert,
		caKey:       caKey,
		certPool:    pool,
		validPeriod: validPeriod,
	}, nil
}

func (m manager) GetCACertPEM() []byte {
	return m.caCertPEM
}

func (m manager) GetCACert() *x509.Certificate {
	return m.caCert
}

func (m manager) NewCertKey(cfg Config) ([]byte, []byte, error) {
	if cfg.ValidityPeriod > m.validPeriod {
		cfg.ValidityPeriod = m.validPeriod
	}

	return NewCertFromCA(m.caCert, m.caKey, cfg)
}

func (m manager) SignCert(csr []byte) ([]byte, error) {
	req, err := x509.ParseCertificateRequest(csr)
	if err != nil {
		return nil, err
	}

	template, err := buildCertTemplate(Config{
		CommonName:     req.Subject.CommonName,
		Organization:   req.Subject.Organization,
		DNSNames:       req.DNSNames,
		IPs:            req.IPAddresses,
		ValidityPeriod: m.validPeriod,
		Usages:         ExtKeyUsagesServerAndClient,
	})
	if err != nil {
		return nil, err
	}

	return x509.CreateCertificate(rand.Reader, template, m.caCert, req.PublicKey, m.caKey)
}

func (m manager) VerifyCert(cert *x509.Certificate, usages []x509.ExtKeyUsage) error {
	opts := x509.VerifyOptions{
		Roots:     m.certPool,
		KeyUsages: usages,
	}

	_, err := cert.Verify(opts)
	return err
}

func (m manager) VerifyCertInPEM(certPEM []byte, usages []x509.ExtKeyUsage) error {
	block, _ := pem.Decode(certPEM)

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	return m.VerifyCert(cert, usages)
}
