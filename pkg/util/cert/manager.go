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
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

type Manager interface {
	// SignCert Create a cert/key pair from CA with specified config
	SignCert(cfg Config) (certDER []byte, keyDER []byte, err error)
	VerifyCert(cert *x509.Certificate, usages []x509.ExtKeyUsage) error
	VerifyCertInPEM(certPEM []byte, usages []x509.ExtKeyUsage) error
	GetCACertPEM() []byte
}

type manager struct {
	caCertPEM []byte
	caCert    *x509.Certificate
	caKey     *rsa.PrivateKey
	certPool  *x509.CertPool
}

func NewManger(caDER, caKeyDER []byte) (Manager, error) {
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
		caCertPEM: EncodeCertPEM(caDER),
		caCert:    caCert,
		caKey:     caKey,
		certPool:  pool,
	}, nil
}

func (m manager) GetCACertPEM() []byte {
	return m.caCertPEM
}

func (m manager) SignCert(cfg Config) ([]byte, []byte, error) {
	return NewCertFromCA(m.caCert, m.caKey, cfg)
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
