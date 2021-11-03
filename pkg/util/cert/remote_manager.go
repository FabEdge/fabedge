package cert

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// SignCertFunc receive csr and return a cert bytes
type SignCertFunc func(csr []byte) ([]byte, error)

type remoteManager struct {
	caCert   *x509.Certificate
	signCert SignCertFunc

	caCertPEM []byte
	certPool  *x509.CertPool
}

func NewRemoteManager(caCertDER []byte, signCert SignCertFunc) (Manager, error) {
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse a caCert. err: %v", err)
	}

	if signCert == nil {
		return nil, fmt.Errorf("a signCert function is required")
	}

	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	return &remoteManager{
		caCertPEM: EncodeCertPEM(caCertDER),
		caCert:    caCert,
		certPool:  pool,
		signCert:  signCert,
	}, nil
}

func (m remoteManager) GetCACertPEM() []byte {
	return m.caCertPEM
}

func (m remoteManager) GetCACert() *x509.Certificate {
	return m.caCert
}

func (m remoteManager) NewCertKey(cfg Config) ([]byte, []byte, error) {
	keyDER, csr, err := NewCertRequest(Request{
		CommonName:   cfg.CommonName,
		Organization: cfg.Organization,
		IPs:          cfg.IPs,
		DNSNames:     cfg.DNSNames,
	})

	certDER, err := m.signCert(csr)

	return certDER, keyDER, err
}

func (m remoteManager) SignCert(csr []byte) ([]byte, error) {
	return m.signCert(csr)
}

func (m remoteManager) VerifyCert(cert *x509.Certificate, usages []x509.ExtKeyUsage) error {
	opts := x509.VerifyOptions{
		Roots:     m.certPool,
		KeyUsages: usages,
	}

	_, err := cert.Verify(opts)
	return err
}

func (m remoteManager) VerifyCertInPEM(certPEM []byte, usages []x509.ExtKeyUsage) error {
	block, _ := pem.Decode(certPEM)

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	return m.VerifyCert(cert, usages)
}
