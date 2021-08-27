package cert

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/fabedge/fabedge/pkg/common/about"
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
		Long:  "Create a self-signed CA cert/key pair, by default data will be save to a secret specified by '-ca-secret' flag",
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
fabedge-cert gen edge --name=edge-ck

# Create a cert/key pair using commonName edge but save them to files
fabedge-cert gen edge --save-to-file --save-to-secret=false
`,
		Args:    cobra.MinimumNArgs(1),
		PreRunE: doValidations(certOptions.Validate),
		Run: func(cmd *cobra.Command, args []string) {
			caDER, caKeyDER := getCA(globalOptions)

			commonName := args[0]
			usages := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
			cfg := certOptions.AsConfig(commonName, false, usages)

			certDER, keyDER, err := certutil.NewCertFromCA2(caDER, caKeyDER, cfg)
			if err != nil {
				exit("failed to create cert/key from ca: %s", err)
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
			caDER, _ := getCA(globalOptions)

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
	createOrUpdateSecret(&corev1.Secret{
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
