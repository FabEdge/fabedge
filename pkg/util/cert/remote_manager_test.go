package cert_test

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math"
	"math/big"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	certutil "github.com/fabedge/fabedge/pkg/util/cert"
)

var _ = Describe("RemoteManager", func() {
	var manager certutil.Manager

	BeforeEach(func() {
		caDER, caKeyDER, err := certutil.NewSelfSignedCA(certutil.Config{
			CommonName:     certutil.DefaultCAName,
			Organization:   []string{certutil.DefaultOrganization},
			IsCA:           true,
			ValidityPeriod: 24 * time.Hour,
		})
		Expect(err).Should(BeNil())

		caCert, err := x509.ParseCertificate(caDER)
		Expect(err).Should(BeNil())

		privateKey, err := x509.ParsePKCS1PrivateKey(caKeyDER)
		Expect(err).Should(BeNil())

		signCert := func(csr []byte) ([]byte, error) {
			req, err := x509.ParseCertificateRequest(csr)
			if err != nil {
				return nil, err
			}

			serialNumber, _ := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
			template := x509.Certificate{
				Subject: pkix.Name{
					CommonName:   req.Subject.CommonName,
					Organization: req.Subject.Organization,
					Country:      []string{certutil.DefaultCountry},
				},
				DNSNames:     req.DNSNames,
				IPAddresses:  req.IPAddresses,
				SerialNumber: serialNumber,
				KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
				ExtKeyUsage:  certutil.ExtKeyUsagesServerAndClient,
				NotBefore:    time.Now().UTC(),
				NotAfter:     time.Now().Add(time.Hour),

				BasicConstraintsValid: false,
				IsCA:                  false,

				PublicKeyAlgorithm: x509.RSA,
				SignatureAlgorithm: x509.SHA384WithRSA,
			}
			if err != nil {
				return nil, err
			}

			return x509.CreateCertificate(rand.Reader, &template, caCert, req.PublicKey, privateKey)
		}

		manager, err = certutil.NewRemoteManager(caDER, signCert)
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
		Expect(err).Should(BeNil())
		Expect(cert.Subject.CommonName).Should(Equal(cfg.CommonName))
		Expect(cert.Subject.Organization).Should(Equal(cfg.Organization))
		Expect(cert.DNSNames).Should(Equal(cfg.DNSNames))
		Expect(cert.IPAddresses).Should(Equal(cfg.IPs))
		Expect(cert.IsCA).Should(BeFalse())
		Expect(cert.KeyUsage).Should(Equal(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature))
		Expect(cert.ExtKeyUsage).Should(Equal(certutil.ExtKeyUsagesServerAndClient))
		Expect(manager.VerifyCert(cert, cfg.Usages)).Should(Succeed())
		Expect(manager.VerifyCertInPEM(certutil.EncodeCertPEM(certDER), cfg.Usages)).Should(Succeed())
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
