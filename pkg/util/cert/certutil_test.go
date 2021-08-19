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

		manager, err := certutil.NewManger(caDER, caKey)
		Expect(err).ShouldNot(HaveOccurred())

		edgeCfg := certutil.Config{
			CommonName:     "edge",
			Organization:   []string{certutil.DefaultOrganization},
			IPs:            []net.IP{net.ParseIP("2.2.2.2")},
			Usages:         []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			ValidityPeriod: 24 * time.Hour,
		}
		edgeDER, _, err := manager.SignCert(edgeCfg)
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
})
