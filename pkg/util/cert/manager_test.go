package cert_test

import (
	"crypto/x509"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	certutil "github.com/fabedge/fabedge/pkg/util/cert"
)

var _ = Describe("Manager", func() {
	var manager certutil.Manager

	BeforeEach(func() {
		caDER, keyDER, err := certutil.NewSelfSignedCA(certutil.Config{
			CommonName:     certutil.DefaultCAName,
			Organization:   []string{certutil.DefaultOrganization},
			IsCA:           true,
			ValidityPeriod: 24 * time.Hour,
		})
		Expect(err).Should(BeNil())

		manager, err = certutil.NewManger(caDER, keyDER, 24*time.Hour)
		Expect(err).Should(BeNil())
	})

	It("should be able to create cert/key pair", func() {
		cfg := certutil.Config{
			CommonName:     "test",
			Organization:   []string{certutil.DefaultOrganization},
			IsCA:           false,
			ValidityPeriod: 24 * time.Hour,
			Usages:         certutil.ExtKeyUsagesServerAndClient,
		}
		certDER, keyDER, err := manager.NewCertKey(cfg)

		_, err = x509.ParsePKCS1PrivateKey(keyDER)
		Expect(err).Should(BeNil())

		cert, err := x509.ParseCertificate(certDER)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(cert.Subject.CommonName).Should(Equal(cfg.CommonName))
		Expect(cert.Subject.Organization).Should(Equal(cfg.Organization))
		Expect(cert.Subject.Country).Should(Equal([]string{certutil.DefaultCountry}))
		Expect(cert.DNSNames).Should(Equal(cfg.DNSNames))
		Expect(cert.IPAddresses).Should(Equal(cfg.IPs))
		Expect(cert.IsCA).Should(BeFalse())
		Expect(cert.KeyUsage).Should(Equal(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature))
		Expect(cert.ExtKeyUsage).Should(Equal(certutil.ExtKeyUsagesServerAndClient))
		Expect(manager.VerifyCert(cert, cfg.Usages)).Should(Succeed())
		Expect(manager.VerifyCertInPEM(certutil.EncodeCertPEM(certDER), cfg.Usages)).Should(Succeed())
	})

	It("should use validPeriod of config passed if the validPeriod is less than validPeriod of manager", func() {
		cfg := certutil.Config{
			CommonName:     "test",
			Organization:   []string{certutil.DefaultOrganization},
			IsCA:           false,
			ValidityPeriod: time.Second,
			Usages:         certutil.ExtKeyUsagesServerAndClient,
		}
		certDER, _, err := manager.NewCertKey(cfg)
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(time.Second)
		cert, err := x509.ParseCertificate(certDER)
		Expect(manager.VerifyCert(cert, cfg.Usages)).Should(HaveOccurred())
	})

	It("should be able to create cert from certificate request", func() {
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

		certDER, err := manager.SignCert(csr)
		Expect(err).Should(BeNil())

		cert, err := x509.ParseCertificate(certDER)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(cert.PublicKey).Should(Equal(privateKey.Public()))
		Expect(cert.Subject.CommonName).Should(Equal(req.CommonName))
		Expect(cert.Subject.Organization).Should(Equal(req.Organization))
		Expect(cert.Subject.Country).Should(Equal([]string{certutil.DefaultCountry}))
		Expect(cert.DNSNames).Should(Equal(req.DNSNames))
		Expect(cert.IPAddresses[0].Equal(req.IPs[0])).Should(BeTrue())
		Expect(cert.IsCA).Should(BeFalse())
		Expect(cert.KeyUsage).Should(Equal(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature))
		Expect(cert.ExtKeyUsage).Should(Equal(certutil.ExtKeyUsagesServerAndClient))
		Expect(manager.VerifyCert(cert, certutil.ExtKeyUsagesServerAndClient)).Should(Succeed())
		Expect(manager.VerifyCertInPEM(certutil.EncodeCertPEM(certDER), certutil.ExtKeyUsagesServerAndClient)).Should(Succeed())
	})
})
