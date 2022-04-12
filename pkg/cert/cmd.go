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
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/fabedge/fabedge/pkg/common/about"
	fclient "github.com/fabedge/fabedge/pkg/operator/client"
	certutil "github.com/fabedge/fabedge/pkg/util/cert"
	secretutil "github.com/fabedge/fabedge/pkg/util/secret"
)

func NewCertCommand() *cobra.Command {
	var globalOptions = &GlobalOptions{}
	var saveOptions = &SaveOptions{}
	var certOptions = &CertOptions{}
	var verifyOptions = &VerifyOptions{}

	caCmd := &cobra.Command{
		Use:   "ca [CommonName]",
		Short: "Create a self-signed CA cert/key pair",
		Long:  "Create a self-signed CA cert/key pair, by default data will be save to a secret specified by '-ca-secret' flag. CA cert/key pair could not be created remotely",
		Example: `# Create a self-signed CA with default commonName to secret fabedge-ca
fabedge-cert gen ca

# Create a self-signed CA with specified commonName
fabedge-cert gen ca my-ca

# Create a self-signed CA to secret ca in namespace default
fabedge-cert gen ca --ca-secret=ca --namespace=default

# Create a self-signed CA and save to files only
fabedge-cert gen ca --save-to-file --save-to-secret=false
`,
		Args:    cobra.MaximumNArgs(1),
		PreRunE: doValidations(certOptions.Validate),
		Run: func(cmd *cobra.Command, args []string) {
			name, secretName := certutil.DefaultCAName, globalOptions.CASecret
			if len(args) > 0 {
				name = args[0]
			}

			cfg := certOptions.AsConfig(name, true, nil)
			certDER, keyDER, err := certutil.NewSelfSignedCA(cfg)
			if err != nil {
				exit("failed to generate CA cert/key: %s", err)
			}

			if saveOptions.Secret {
				saveCAToSecret(secretName, globalOptions.Namespace, certDER, keyDER)
			}
		},
	}

	genCmd := &cobra.Command{
		Use:   "gen commonName",
		Short: "Create a cert/key pair from specified ca",
		Long: `Create a cert/key pair from specified ca. By default the cert and key will be save to a secret named by your commonName, 
you can specify the target secret name. You can also choose to save cert/key pair to files.
`,
		Example: `# Create a cert/key pair using commonName edge
fabedge-cert gen edge

# Create a cert/key pair using commonName edge and save data to secret edge-ck
fabedge-cert gen edge --secret-name=edge-ck

# Create a cert/key pair using commonName edge but save them to files
fabedge-cert gen edge --save-to-file --save-to-secret=false
`,
		Args:    cobra.MinimumNArgs(1),
		PreRunE: doValidations(certOptions.Validate),
		Run: func(cmd *cobra.Command, args []string) {
			var (
				caDER      []byte
				certDER    []byte
				keyDER     []byte
				err        error
				commonName = args[0]
			)

			if !globalOptions.Remote {
				var caKeyDER []byte
				caDER, caKeyDER = getCA(globalOptions)

				usages := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
				cfg := certOptions.AsConfig(commonName, false, usages)
				certDER, keyDER, err = certutil.NewCertFromCA2(caDER, caKeyDER, cfg)
				if err != nil {
					exit("failed to create cert/key from ca: %s", err)
				}
			} else {
				cacert, err := fclient.GetCertificate(globalOptions.APIServerAddress)
				if err != nil {
					exit("failed to get CA cert from host cluster: %s", err)
				}

				var csrDER []byte
				keyDER, csrDER, err = certutil.NewCertRequest(certOptions.AsRequest(commonName))
				if err != nil {
					exit("failed to create certificate request: %s", err)
				}

				certPool := x509.NewCertPool()
				certPool.AddCert(cacert.Raw)
				cert, err := fclient.SignCertByToken(globalOptions.APIServerAddress, globalOptions.Token, csrDER, certPool)
				if err != nil {
					exit("failed to create certificate: %s", err)
				}
				caDER = cacert.DER
				certDER = cert.DER
			}

			if saveOptions.Secret {
				secretName := commonName
				if len(saveOptions.Name) != 0 {
					secretName = saveOptions.Name
				}
				saveCertAndKeyToSecret(secretName, globalOptions.Namespace, caDER, certDER, keyDER)
			}

			if saveOptions.File {
				saveCertAndKey(commonName, certDER, keyDER)
			}
		},
	}

	saveCaCmd := &cobra.Command{
		Use:     "save-ca",
		Short:   "Save your ca cert/key from files to secret",
		Example: `fabedge-cert save-ca --ca-secret=fabedge-ca --ca-cert=ca.crt --ca-key=ca.key`,
		Args:    cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			datader, err := certutil.ReadPEMFileAndDecode(globalOptions.CACert)
			if err != nil {
				exit("failed to read file %s: %s", globalOptions.CACert, err)
			}

			keyder, err := certutil.ReadPEMFileAndDecode(globalOptions.CAKey)
			if err != nil {
				exit("failed to read file %s: %s", globalOptions.CAKey, err)
			}

			saveCAToSecret(globalOptions.CASecret, globalOptions.Namespace, datader, keyder)
		},
	}

	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Verity your certs against specified CA",
		Example: `
fabedge-cert verify --secret edge
fabedge-cert verify --file edge.crt
fabedge-cert verify --secret edge --ca-secret=fabedge-ca --namespace=fabedge
`,
		PreRunE: doValidations(verifyOptions.Validate),
		Run: func(cmd *cobra.Command, args []string) {
			var caDER []byte

			if globalOptions.Remote {
				cacert, err := fclient.GetCertificate(globalOptions.APIServerAddress)
				if err != nil {
					exit("failed to get CA cert from host cluster: %s", err)
				}
				caDER = cacert.DER
			} else {
				caDER, _ = getCA(globalOptions)
			}

			var certDER []byte
			if len(verifyOptions.Secret) > 0 {
				key := client.ObjectKey{
					Name:      verifyOptions.Secret,
					Namespace: globalOptions.Namespace,
				}

				caDER2, _ := getCertAndKeyFromSecret(key, secretutil.KeyCACert, corev1.TLSPrivateKeyKey)
				if !bytes.Equal(caDER2, caDER) {
					exit("The ca.crt is different from the ca.crt in ca secret")
				}

				certDER, _ = getCertAndKeyFromSecret(key, corev1.TLSCertKey, corev1.TLSPrivateKeyKey)
			}

			if len(verifyOptions.File) > 0 {
				data, err := certutil.ReadPEMFileAndDecode(verifyOptions.File)
				if err != nil {
					exit("failed to read file %s: %s", verifyOptions.File, err)
				}

				certDER = data
			}

			usages := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
			err := certutil.VerifyCert(caDER, certDER, usages)
			if err != nil {
				exit("%s", err)
			} else {
				fmt.Println("Your cert is ok")
			}
		},
	}

	viewCertCmd := &cobra.Command{
		Use:   "view secretName",
		Short: "View the certificate of a TLS secret",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := client.ObjectKey{Name: args[0], Namespace: globalOptions.Namespace}
			cert := getCertificateFromSecret(key)
			fmt.Printf("Version: %d\n", cert.Version)
			fmt.Printf("Subject: %s\n", cert.Subject)
			fmt.Printf("Issuer: %s\n", cert.Issuer)
			fmt.Printf("IsCA: %t\n", cert.IsCA)
			fmt.Printf("Signature Algorithm: %s\n", cert.SignatureAlgorithm)
			fmt.Printf("Publickey Algorithm: %s\n", cert.PublicKeyAlgorithm)
			fmt.Printf("Validity: \n")
			fmt.Printf("      Not Before: %s\n", cert.NotBefore)
			fmt.Printf("      Not After: %s\n", cert.NotAfter)
			fmt.Printf("Key length: %d\n", cert.PublicKey.(*rsa.PublicKey).Size()*8)
			fmt.Printf("Key Usage: %s\n", formatKeyUsage(cert.KeyUsage))
			fmt.Printf("Ext Key Usage: %s\n", formatExtUsages(cert.ExtKeyUsage))
			fmt.Printf("DNS Names: %s\n", strings.Join(cert.DNSNames, " "))
			fmt.Printf("IP Addresses: %s\n", formatIPs(cert.IPAddresses))
			fmt.Printf("Email Addresses: %s\n", strings.Join(cert.EmailAddresses, " "))
			fmt.Printf("URIs: %s\n", formatURIs(cert.URIs))
		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Run: func(cmd *cobra.Command, args []string) {
			about.DisplayVersion()
		},
	}

	var rootCmd = &cobra.Command{
		Use:   "fabedge-cert",
		Short: "A cert generator for fabedge to facilitate deploying connector and agents",
	}

	saveOptions.AddFlags(genCmd.PersistentFlags())
	certOptions.AddFlags(genCmd.PersistentFlags())
	verifyOptions.AddFlags(verifyCmd.Flags())
	globalOptions.AddFlags(rootCmd.PersistentFlags())

	genCmd.AddCommand(caCmd)
	rootCmd.AddCommand(
		genCmd,
		saveCaCmd,
		verifyCmd,
		viewCertCmd,
		versionCmd,
	)

	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	return rootCmd
}

func exit(format string, a ...interface{}) {
	fmt.Printf(format+"\n", a...)
	os.Exit(1)
}

func saveCertAndKey(cn string, certDER, keyDER []byte) {
	certFile, keyFile := fmt.Sprintf("%s.crt", cn), fmt.Sprintf("%s.key", cn)

	err := certutil.SaveCertKeyToFile(certDER, keyDER, certFile, keyFile)
	if err != nil {
		exit("failed to save cert/key: %s", err)
	}

	fmt.Printf("cert/key are saved to %s and %s\n", certFile, keyFile)
}

func createKubeClient() client.Client {
	cfg, err := config.GetConfig()
	if err != nil {
		exit("not able to initiate kube client config: %s", err)
	}

	cli, err := client.New(cfg, client.Options{})
	if err != nil {
		exit("not able to create kube client: %s", err)
	}

	return cli
}

func saveCAToSecret(name, namespace string, caDER, keyDER []byte) {
	createSecretIfNotExist(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			secretutil.KeyCACert: certutil.EncodeCertPEM(caDER),
			secretutil.KeyCAKey:  certutil.EncodePrivateKeyPEM(keyDER),
		},
	})
}

func saveCertAndKeyToSecret(name, namespace string, caCertDER, certDER, keyDER []byte) {
	secret := secretutil.TLSSecret().
		Name(name).
		Namespace(namespace).
		EncodeCACert(caCertDER).
		EncodeCert(certDER).
		EncodeKey(keyDER).
		Build()
	createOrUpdateSecret(&secret)
}

func createOrUpdateSecret(secret *corev1.Secret) {
	cli := createKubeClient()

	err := cli.Create(context.TODO(), secret)
	switch {
	case err == nil:
		fmt.Printf("secret %s/%s is saved\n", secret.Namespace, secret.Name)
		return
	case errors.IsAlreadyExists(err):
		if err := cli.Update(context.TODO(), secret); err != nil {
			exit("failed to save secret: %s", err)
		}
		fmt.Printf("secret %s/%s is saved\n", secret.Namespace, secret.Name)
	default:
		exit("failed to save secret: %s", err)
	}
}

func createSecretIfNotExist(secret *corev1.Secret) {
	cli := createKubeClient()

	err := cli.Create(context.TODO(), secret)
	switch {
	case err == nil:
		fmt.Printf("secret %s/%s is saved\n", secret.Namespace, secret.Name)
		return
	case errors.IsAlreadyExists(err):
		fmt.Printf("secret %s/%s exists and gives up\n", secret.Namespace, secret.Name)
		return
	default:
		exit("failed to save secret: %s", err)
	}
}

func getCA(globalOptions *GlobalOptions) (datader []byte, keyder []byte) {
	if globalOptions.CAIsFromSecret() {
		return getCertAndKeyFromSecret(globalOptions.SecretKey(), secretutil.KeyCACert, secretutil.KeyCAKey)
	}

	datader, err := certutil.ReadPEMFileAndDecode(globalOptions.CACert)
	if err != nil {
		exit("failed to read file %s: %s", globalOptions.CACert, err)
	}

	keyder, err = certutil.ReadPEMFileAndDecode(globalOptions.CAKey)
	if err != nil {
		exit("failed to read file %s: %s", globalOptions.CAKey, err)
	}

	return datader, keyder
}

func getCertAndKeyFromSecret(key client.ObjectKey, certName, keyName string) (certDER []byte, keyDER []byte) {
	var secret corev1.Secret
	err := createKubeClient().Get(context.TODO(), key, &secret)
	if err != nil {
		exit("failed to get secret: %s", err)
	}

	return decodePEM(secret.Data[certName]), decodePEM(secret.Data[keyName])
}

func getCertificateFromSecret(key client.ObjectKey) *x509.Certificate {
	var secret corev1.Secret
	err := createKubeClient().Get(context.TODO(), key, &secret)
	if err != nil {
		exit("failed to get secret: %s", err)
	}

	pemBytes := secret.Data[corev1.TLSCertKey]
	if len(pemBytes) == 0 {
		pemBytes = secret.Data[secretutil.KeyCACert]
	}

	cert, err := x509.ParseCertificate(decodePEM(pemBytes))
	if err != nil {
		exit("failed to decode certificate: %s", err)
	}

	return cert

}

func decodePEM(data []byte) []byte {
	block, _ := pem.Decode(data)
	if block == nil {
		exit("failed to decode pem data")
	}

	return block.Bytes
}

func doValidations(validateFns ...func() error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		for _, validate := range validateFns {
			if err := validate(); err != nil {
				return err
			}
		}

		return nil
	}
}

func formatKeyUsage(keyUsage x509.KeyUsage) string {
	var usages []string
	isUsed := func(expected x509.KeyUsage) bool {
		return keyUsage&expected == expected
	}

	if isUsed(x509.KeyUsageDigitalSignature) {
		usages = append(usages, "DigitalSignature")
	}

	if isUsed(x509.KeyUsageContentCommitment) {
		usages = append(usages, "ContentCommitment")
	}

	if isUsed(x509.KeyUsageKeyEncipherment) {
		usages = append(usages, "KeyEncipherment")
	}

	if isUsed(x509.KeyUsageDataEncipherment) {
		usages = append(usages, "DataEncipherment")
	}

	if isUsed(x509.KeyUsageKeyAgreement) {
		usages = append(usages, "KeyAgreement")
	}

	if isUsed(x509.KeyUsageCertSign) {
		usages = append(usages, "CertSign")
	}

	if isUsed(x509.KeyUsageCRLSign) {
		usages = append(usages, "CRLSign")
	}

	if isUsed(x509.KeyUsageEncipherOnly) {
		usages = append(usages, "EncipherOnly")
	}

	if isUsed(x509.KeyUsageDecipherOnly) {
		usages = append(usages, "DecipherOnly")
	}

	return strings.Join(usages, " ")
}

func formatExtUsages(extKeyUsages []x509.ExtKeyUsage) string {
	var usages []string

	for _, eku := range extKeyUsages {
		t := ""
		switch eku {
		case x509.ExtKeyUsageAny:
			t = "Any"
		case x509.ExtKeyUsageServerAuth:
			t = "ServerAuth"
		case x509.ExtKeyUsageClientAuth:
			t = "ClientAuth"
		case x509.ExtKeyUsageCodeSigning:
			t = "CodeSigning"
		case x509.ExtKeyUsageEmailProtection:
			t = "EmailProtection"
		case x509.ExtKeyUsageIPSECEndSystem:
			t = "IPSECEndSystem"
		case x509.ExtKeyUsageIPSECTunnel:
			t = "IPSECTunnel"
		case x509.ExtKeyUsageIPSECUser:
			t = "IPSECUser"
		case x509.ExtKeyUsageTimeStamping:
			t = "TimeStamping"
		case x509.ExtKeyUsageOCSPSigning:
			t = "OCSPSigning"
		case x509.ExtKeyUsageMicrosoftServerGatedCrypto:
			t = "MicrosoftServerGatedCrypto"
		case x509.ExtKeyUsageNetscapeServerGatedCrypto:
			t = "NetscapeServerGatedCrypto"
		case x509.ExtKeyUsageMicrosoftCommercialCodeSigning:
			t = "MicrosoftCommercialCodeSigning"
		case x509.ExtKeyUsageMicrosoftKernelCodeSigning:
			t = "MicrosoftKernelCodeSigning"
		}

		if t != "" {
			usages = append(usages, t)
		}
	}

	return strings.Join(usages, " ")
}

func formatIPs(ips []net.IP) string {
	var values []string
	for _, ip := range ips {
		values = append(values, ip.String())
	}

	return strings.Join(values, " ")
}

func formatURIs(urls []*url.URL) string {
	var values []string
	for _, url := range urls {
		values = append(values, url.String())
	}

	return strings.Join(values, " ")
}
