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

package cert_test

import (
	"crypto/x509"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	certutil "github.com/fabedge/fabedge/pkg/util/cert"
)

var _ = Describe("Certutil", func() {
	caCfg := certutil.Config{
		CommonName:     certutil.DefaultCAName,
		Organization:   []string{certutil.DefaultOrganization},
		IsCA:           true,
		ValidityPeriod: 24 * time.Hour,
	}

	It("should create ca cert/key pair from config", func() {
		caDER, _, err := certutil.NewSelfSignedCA(caCfg)
		Expect(err).ShouldNot(HaveOccurred())

		caCert, err := x509.ParseCertificate(caDER)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(caCert.Subject.CommonName).Should(Equal(caCfg.CommonName))
		Expect(caCert.Subject.Organization).Should(Equal(caCfg.Organization))
		Expect(caCert.Subject.Country).Should(Equal([]string{certutil.DefaultCountry}))
		Expect(caCert.DNSNames).Should(Equal(caCfg.DNSNames))
		Expect(caCert.IPAddresses).Should(Equal(caCfg.IPs))
		Expect(caCert.BasicConstraintsValid).Should(BeTrue())
		Expect(caCert.IsCA).Should(BeTrue())
		Expect(caCert.KeyUsage).Should(Equal(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign))
		Expect(caCert.ExtKeyUsage).Should(BeEmpty())
	})

	It("should create cert/key pair from specified CA", func() {
		caDER, caKey, err := certutil.NewSelfSignedCA(caCfg)
		Expect(err).ShouldNot(HaveOccurred())

		manager, err := certutil.NewManger(caDER, caKey, 24*time.Hour)
		Expect(err).ShouldNot(HaveOccurred())

		edgeCfg := certutil.Config{
			CommonName:     "edge",
			Organization:   []string{certutil.DefaultOrganization},
			IPs:            []net.IP{net.ParseIP("2.2.2.2")},
			Usages:         []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			ValidityPeriod: 24 * time.Hour,
		}
		edgeDER, _, err := manager.NewCertKey(edgeCfg)
		Expect(err).ShouldNot(HaveOccurred())

		cert, err := x509.ParseCertificate(edgeDER)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(cert.Subject.CommonName).Should(Equal(edgeCfg.CommonName))
		Expect(cert.Subject.Organization).Should(Equal(edgeCfg.Organization))
		Expect(cert.Subject.Country).Should(Equal([]string{certutil.DefaultCountry}))
		Expect(cert.DNSNames).Should(Equal(edgeCfg.DNSNames))
		Expect(cert.IPAddresses[0].Equal(edgeCfg.IPs[0])).Should(BeTrue())
		Expect(cert.BasicConstraintsValid).Should(BeFalse())
		Expect(cert.IsCA).Should(BeFalse())
		Expect(cert.KeyUsage).Should(Equal(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature))
		Expect(cert.ExtKeyUsage).Should(Equal(edgeCfg.Usages))

		Expect(manager.VerifyCert(cert, edgeCfg.Usages)).Should(Succeed())
		Expect(manager.VerifyCertInPEM(certutil.EncodeCertPEM(edgeDER), edgeCfg.Usages)).Should(Succeed())
	})

	It("support create private key and certificate request", func() {
		req := certutil.Request{
			CommonName:   "test",
			Organization: []string{"test"},
			IPs:          []net.IP{net.ParseIP("2.2.2.2")},
			DNSNames:     []string{"www.test.com"},
		}

		keyDER, csr, err := certutil.NewCertRequest(req)
		Expect(err).Should(BeNil())

		privateKey, err := x509.ParsePKCS1PrivateKey(keyDER)
		Expect(err).Should(BeNil())

		cr, err := x509.ParseCertificateRequest(csr)
		Expect(err).Should(BeNil())
		Expect(cr.Subject.CommonName).Should(Equal(req.CommonName))
		Expect(cr.Subject.Organization).Should(Equal(req.Organization))
		Expect(cr.IPAddresses[0].Equal(req.IPs[0])).Should(BeTrue())
		Expect(cr.DNSNames).Should(Equal(req.DNSNames))
		Expect(cr.PublicKey).Should(Equal(privateKey.Public()))
	})
})
