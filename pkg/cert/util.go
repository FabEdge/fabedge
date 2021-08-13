// Copyright 2021 BoCloud
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
	"math"
	"math/big"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	certutil "k8s.io/client-go/util/cert"
	keyutil "k8s.io/client-go/util/keyutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewCA returns CA and CA private key
func NewCA() ([]byte, []byte, error) {
	// generate CA private key
	caKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}

	template, err := BuildCATemplate()
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

func BuildCATemplate() (*x509.Certificate, error) {
	serialNumber, err := GenerateSerialNumber()
	if err != nil {
		return nil, err
	}

	subject := pkix.Name{
		Country:      []string{"CN"},
		Organization: []string{"StrongSwan"},
		CommonName:   "Root CA",
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      subject,
		NotBefore:    time.Now().UTC(),
		NotAfter:     time.Now().Add(time.Hour * 24 * 365 * 100),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,

		PublicKeyAlgorithm: x509.RSA,
		SignatureAlgorithm: x509.SHA384WithRSA,
	}

	return &template, nil
}

func GenerateSerialNumber() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
}

// SignCert creates certificate and key
func SignCert(ca []byte, caKey []byte, commonName string, dnsNames []string, ips []net.IP, validityPeriod time.Duration) ([]byte, []byte, error) {
	// generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate a privateKey, err: %v", err)
	}

	privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey)

	caCert, err := x509.ParseCertificate(ca)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse a caCert from the given ASN.1 DER data, err: %v", err)
	}

	caKeyRSA, err := x509.ParsePKCS1PrivateKey(caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parses an RSA private key in PKCS #1, ASN.1 DER form, err: %v", err)
	}

	certTemplate, err := BuildCertTemplate(commonName, dnsNames, ips, validityPeriod)
	if err != nil {
		return nil, nil, err
	}

	certDER, err := x509.CreateCertificate(rand.Reader, certTemplate, caCert, privateKey.Public(), caKeyRSA)
	if err != nil {
		return nil, nil, err
	}

	return certDER, privateKeyDER, nil

}

func BuildCertTemplate(commonName string, dnsNames []string, ips []net.IP, validityPeriod time.Duration) (*x509.Certificate, error) {
	serialNumber, err := GenerateSerialNumber()
	if err != nil {
		return nil, err
	}
	if len(commonName) == 0 {
		return nil, errors.New("must specify a CommonName")
	}

	subject := pkix.Name{
		Country:      []string{"CN"},
		Organization: []string{"StrongSwan"},
		CommonName:   commonName,
	}

	template := x509.Certificate{
		Subject:            subject,
		DNSNames:           dnsNames,
		IPAddresses:        ips,
		SerialNumber:       serialNumber,
		NotBefore:          time.Now().UTC(),
		NotAfter:           time.Now().Add(time.Hour * 24 * validityPeriod),
		KeyUsage:           x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		PublicKeyAlgorithm: x509.RSA,
		SignatureAlgorithm: x509.SHA384WithRSA,
	}
	return &template, nil
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

func SaveIPSecSecretsToFile(ipsecSecretsContent []byte, ipsecSecretsPath string) error {
	file, err := os.Create(ipsecSecretsPath)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = file.Write(ipsecSecretsContent)
	return err
}

func EncodeCertPEM(certDER []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: certutil.CertificateBlockType, Bytes: certDER})
}

func EncodePrivateKeyPEM(privateKeyDER []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: privateKeyDER})
}

// loadConfig loads a REST Config as per the rules specified in GetConfig
func LoadConfig() (*rest.Config, error) {
	// If a flag is specified with the config location, use that
	if len(kubeConfig) > 0 {
		return clientcmd.BuildConfigFromFlags("", kubeConfig)
	}
	// If an env variable is specified with the config location, use that
	if len(os.Getenv("KUBECONFIG")) > 0 {
		return clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	}
	// If no explicit location, try the in-cluster config
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	// If no in-cluster config, try the default location in the user's home directory
	if usr, err := user.Current(); err == nil {
		if c, err := clientcmd.BuildConfigFromFlags(
			"", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not locate a kubeconfig")
}

func CreateClient() (client.Client, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return client.New(config, client.Options{})
}
