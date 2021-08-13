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
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Manager struct {
	client         client.Client
	namespace      string
	caDER          []byte
	caKeyDER       []byte
	commonName     string
	dnsNames       []string
	ips            []net.IP
	validityPeriod time.Duration
	log            logr.Logger
	saveToFile     bool
}

const (
	caSecretName   = "ca-certs"
	caDataName     = "caDER"
	caKeyDataName  = "caKeyDER"
	caCertDataName = "ca.pem"

	connectorCommonName = "connector"

	ipSecSecretsSecretNameFormat = "{CommonName}-ipsec-secrets"
	ipSecSecretsDataName         = "ipsec.secrets"
	ipSecSecretsDataFormat       = ": RSA {CommonName}_key.pem"

	certSecretNameFormat  = "{CommonName}-certs"
	certDataNameFormat    = "{CommonName}_cert.pem"
	certKeyDataNameFormat = "{CommonName}_key.pem"

	defaultValidityPeriod = 3650
)

func initManager() (*Manager, error) {
	client, err := CreateClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s clientSet, err: %v", err)
	}

	manager := &Manager{
		client:         client,
		namespace:      namespace,
		commonName:     parseCommonNameFlags(),
		dnsNames:       parseDNSNamesFlags(),
		ips:            parseIPAddressesFlags(),
		validityPeriod: time.Duration(validityPeriod),
		log:            klogr.New().WithName("cert"),
		saveToFile:     saveToFile,
	}

	return manager, nil
}

func NewManager(client client.Client, namespace string) *Manager {
	manager := &Manager{
		client:    client,
		namespace: namespace,
		log:       klogr.New().WithName("cert"),
	}

	return manager
}

func (mgr *Manager) SignConnectorCert() error {
	mgr.log.V(3).Info("starting sign certs for connector", "commonName", mgr.commonName, "dnsNames", mgr.dnsNames, "IPs", mgr.ips)

	if mgr.caDER == nil || mgr.caKeyDER == nil {
		if err := mgr.ensureCASecret(); err != nil {
			return err
		}
	}

	certDER, keyDER, err := SignCert(mgr.caDER, mgr.caKeyDER, mgr.commonName, mgr.dnsNames, mgr.ips, mgr.validityPeriod)
	if err != nil {
		mgr.log.Error(err, "failed to sign cert for connector", "commonName", mgr.commonName, "dnsNames", mgr.dnsNames, "ips", mgr.ips)
		return err
	}

	certSecretName, certDataName, certKeyDataName := mgr.buildCertSecretInfo()

	if mgr.saveToFile {
		if err := SaveCertToFile(mgr.caDER, caCertDataName); err != nil {
			mgr.log.Error(err, "failed to save the CA cert to file", "commonName", mgr.commonName)
			return err
		}

		if err := SaveCertKeyToFile(certDER, keyDER, certDataName, certKeyDataName); err != nil {
			mgr.log.Error(err, "failed to save the connector cert to file", "commonName", mgr.commonName)
			return err
		}

		ipsecSecretsContent := []byte(mgr.buildIPSecSecretsContent(mgr.commonName))
		if err := SaveIPSecSecretsToFile(ipsecSecretsContent, ipSecSecretsDataName); err != nil {
			mgr.log.Error(err, "failed to save the ipsec.secrets content to file", "commonName", mgr.commonName)
			return err
		}
	}

	if err := mgr.createCertsSecret(certDER, keyDER, certSecretName, certDataName, certKeyDataName, mgr.commonName); err != nil {
		mgr.log.Error(err, "failed to create connector-certs secret", "namespace", mgr.namespace, "secretName", certSecretName)
		return err
	}

	if err := mgr.createIPSecSecretsSecret(mgr.commonName); err != nil {
		secretName := strings.Replace(ipSecSecretsSecretNameFormat, "{CommonName}", commonName, 1)
		mgr.log.Error(err, "failed to create secret", "namespace", mgr.namespace, "secretName", secretName)
		return err
	}
	mgr.log.V(3).Info("the connector certificate is successfully issued")
	return nil
}

func (mgr *Manager) ensureCASecret() error {
	if err := mgr.ensureNamespace(); err != nil {
		mgr.log.Error(err, "failed to ensure namespace", "namespace", mgr.namespace)
		return err
	}
	// checkout exist ca secret
	caSecret, err := mgr.existSecret(caSecretName)
	if err != nil {
		return err
	}
	if caSecret != nil {
		mgr.log.V(3).Info("ca-cert secret already exists, skip create CA secret")
		mgr.caDER = caSecret.Data[caDataName]
		mgr.caKeyDER = caSecret.Data[caKeyDataName]
		return nil
	}

	mgr.log.V(3).Info("starting create CA cert")

	// new ca
	mgr.caDER, mgr.caKeyDER, err = NewCA()
	if err != nil {
		return fmt.Errorf("failed to create CA cert, err: %v", err)
	}

	// create ca secret
	if err := mgr.createCASecret(); err != nil {
		return fmt.Errorf("failed to create CA cert secret: %s, err: %v", caSecretName, err)
	}

	mgr.log.V(3).Info("the CA certificate is successfully created")

	return nil
}

func (mgr *Manager) ensureNamespace() error {
	err := mgr.client.Get(context.Background(), client.ObjectKey{Name: mgr.namespace}, &corev1.Namespace{})
	if err != nil && errors.IsNotFound(err) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: mgr.namespace,
			},
		}
		if err := mgr.client.Create(context.Background(), ns); err != nil {
			return err
		}
		return nil
	}
	return err
}

func (mgr *Manager) existSecret(secretName string) (*corev1.Secret, error) {
	secret, err := mgr.getSecret(secretName)
	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func (mgr *Manager) getSecret(secretName string) (*corev1.Secret, error) {
	objectKey := client.ObjectKey{
		Namespace: mgr.namespace,
		Name:      secretName,
	}
	secret := &corev1.Secret{}
	if err := mgr.client.Get(context.Background(), objectKey, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func (mgr *Manager) createCASecret() error {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      caSecretName,
			Namespace: mgr.namespace,
		},
		Data: map[string][]byte{
			caDataName:    mgr.caDER,
			caKeyDataName: mgr.caKeyDER,
		},
		Type: corev1.SecretTypeOpaque,
	}
	return mgr.createSecret(&secret)
}

func (mgr *Manager) createSecret(secret *corev1.Secret) error {
	if err := mgr.client.Create(context.Background(), secret); err != nil {
		if errors.IsAlreadyExists(err) {
			oldSecret, err := mgr.getSecret(secret.Name)
			if err != nil {
				return err
			}
			if err := mgr.client.Update(context.Background(), oldSecret); err != nil {
				return fmt.Errorf("failed to update the secret, namespace: %s, name: %s, err: %v", mgr.namespace, secret.Name, err)
			}
		} else {
			return fmt.Errorf("failed to create the secret, namespace: %s, name: %s, err: %v", mgr.namespace, secret.Name, err)
		}
	}
	return nil
}

func (mgr *Manager) buildCertSecretInfo() (string, string, string) {
	certSecretName := strings.Replace(certSecretNameFormat, "{CommonName}", mgr.commonName, 1)
	certDataName := strings.Replace(certDataNameFormat, "{CommonName}", mgr.commonName, 1)
	certKeyDataName := strings.Replace(certKeyDataNameFormat, "{CommonName}", mgr.commonName, 1)

	return certSecretName, certDataName, certKeyDataName
}

func (mgr *Manager) GetCertSecretDataInfo() (string, string, string, string) {
	certSecretName, certDataName, certKeyDataName := mgr.buildCertSecretInfo()
	return certSecretName, caCertDataName, certDataName, certKeyDataName
}

func (mgr *Manager) SignEdgeCertAndPersistence() error {
	mgr.log.V(3).Info("starting sign certs for edge", "commonName", mgr.commonName, "dnsNames", mgr.dnsNames, "IPs", mgr.ips)
	if mgr.caDER == nil || mgr.caKeyDER == nil {
		caSecret, err := mgr.getSecret(caSecretName)
		if err != nil {
			mgr.log.Error(err, "failed to get ca-certs secret", "namespace", mgr.namespace, "secretName", caSecretName)
			return fmt.Errorf("failed to sign edge certs, error: %v", err)
		}
		mgr.caDER = caSecret.Data[caDataName]
		mgr.caKeyDER = caSecret.Data[caKeyDataName]
	}

	certDER, keyDER, err := SignCert(mgr.caDER, mgr.caKeyDER, mgr.commonName, mgr.dnsNames, mgr.ips, mgr.validityPeriod)
	if err != nil {
		mgr.log.Error(err, "failed to sign cert for edge", "commonName", mgr.commonName, "dnsNames", mgr.dnsNames, "IPs", mgr.ips)
		return err
	}

	certSecretName, certDataName, certKeyDataName := mgr.buildCertSecretInfo()

	if mgr.saveToFile {
		if err := SaveCertToFile(mgr.caDER, caCertDataName); err != nil {
			mgr.log.Error(err, "failed to save the CA cert to file", "commonName", mgr.commonName)
			return err
		}
		if err := SaveCertKeyToFile(certDER, keyDER, certDataName, certKeyDataName); err != nil {
			mgr.log.Error(err, "failed to save the edge cert to file", "commonName", mgr.commonName)
			return err
		}

		ipsecSecretsContent := []byte(mgr.buildIPSecSecretsContent(mgr.commonName))
		if err := SaveIPSecSecretsToFile(ipsecSecretsContent, ipSecSecretsDataName); err != nil {
			mgr.log.Error(err, "failed to save the ipsec.secrets content to file", "commonName", mgr.commonName)
			return err
		}
	}

	if err := mgr.createCertsSecret(certDER, keyDER, certSecretName, certDataName, certKeyDataName, mgr.commonName); err != nil {
		mgr.log.Error(err, "failed to create edge certs secret", "namespace", mgr.namespace, "secretName", certSecretName)
		return err
	}

	if err := mgr.createIPSecSecretsSecret(mgr.commonName); err != nil {
		secretName := strings.Replace(ipSecSecretsSecretNameFormat, "{CommonName}", commonName, 1)
		mgr.log.Error(err, "failed to create secret", "namespace", mgr.namespace, "secretName", secretName)
		return err
	}

	mgr.log.V(3).Info("the edge certificate is successfully issued", "commonName", mgr.commonName)
	return nil
}

func (mgr *Manager) createCertsSecret(certDER, keyDER []byte, certSecretName, certDataName, certKeyDataName, commonName string) error {
	caPemBytes := EncodeCertPEM(mgr.caDER)
	certPemBytes := EncodeCertPEM(certDER)
	keyPemBytes := EncodePrivateKeyPEM(keyDER)

	if caPemBytes == nil || certPemBytes == nil || keyPemBytes == nil {
		return fmt.Errorf("failed to encoded connector certs. caDER or certDER or keyDER has invalid headers")
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certSecretName,
			Namespace: mgr.namespace,
		},
		StringData: map[string]string{
			caCertDataName:  string(caPemBytes),
			certDataName:    string(certPemBytes),
			certKeyDataName: string(keyPemBytes),
		},
		Type: corev1.SecretTypeOpaque,
	}
	return mgr.createSecret(&secret)
}

func (mgr *Manager) createIPSecSecretsSecret(commonName string) error {
	secretName := strings.Replace(ipSecSecretsSecretNameFormat, "{CommonName}", commonName, 1)
	data := mgr.buildIPSecSecretsContent(commonName)

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: mgr.namespace,
		},
		StringData: map[string]string{
			ipSecSecretsDataName: data,
		},
		Type: corev1.SecretTypeOpaque,
	}
	return mgr.createSecret(&secret)
}

func (mgr *Manager) buildIPSecSecretsContent(commonName string) string {
	return strings.Replace(ipSecSecretsDataFormat, "{CommonName}", commonName, 1)
}

func (mgr *Manager) SignEdgeCert(commonName string, dnsNames []string, ips []net.IP, validityPeriod int) (caDER, certDER, keyDER, ipsecSecrets []byte, err error) {
	mgr.log.V(3).Info("starting sign certs for edge", "commonName", commonName, "dnsNames", dnsNames, "IPs", ips)

	if len(commonName) == 0 {
		err = fmt.Errorf("the value of common name is not expected to be empty")
		return
	}

	if validityPeriod <= 0 {
		mgr.log.V(3).Info("set cert validity period to defaultValidityPeriod", "validityPeriod", validityPeriod, "defaultValidityPeriod", defaultValidityPeriod)
		validityPeriod = defaultValidityPeriod
	}

	if len(dnsNames) == 0 {
		dnsNames = append(dnsNames, commonName)
	}

	if mgr.caDER == nil || mgr.caKeyDER == nil {
		var caSecret *corev1.Secret
		caSecret, err = mgr.getSecret(caSecretName)
		if err != nil {
			mgr.log.Error(err, "failed to get ca-certs secret", "namespace", mgr.namespace, "secretName", caSecretName)
			err = fmt.Errorf("failed to sign edge certs, error: %v", err)
			return
		}

		mgr.caDER = caSecret.Data[caDataName]
		mgr.caKeyDER = caSecret.Data[caKeyDataName]
	}

	certDER, keyDER, err = SignCert(mgr.caDER, mgr.caKeyDER, commonName, dnsNames, ips, time.Duration(validityPeriod))
	caDER = mgr.caDER

	ipsecSecrets = []byte(mgr.buildIPSecSecretsContent(commonName))

	mgr.log.V(3).Info("the edge certificate is successfully issued", "commonName", mgr.commonName)
	return
}
