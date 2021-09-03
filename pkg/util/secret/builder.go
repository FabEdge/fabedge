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

package secret

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	certutil "github.com/fabedge/fabedge/pkg/util/cert"
)

const (
	KeyCACert           = "ca.crt"
	KeyCAKey            = "ca.key"
	KeyIPSecSecretsFile = "ipsec.secrets"
)

type TLSSecretBuilder struct {
	labels      map[string]string
	annotations map[string]string
	name        string
	namespace   string
	cacertPEM   []byte
	certPEM     []byte
	keyPEM      []byte
}

func TLSSecret() *TLSSecretBuilder {
	return &TLSSecretBuilder{}
}

func (b *TLSSecretBuilder) Name(name string) *TLSSecretBuilder {
	b.name = name
	return b
}

func (b *TLSSecretBuilder) Namespace(ns string) *TLSSecretBuilder {
	b.namespace = ns
	return b
}

func (b *TLSSecretBuilder) Label(key, value string) *TLSSecretBuilder {
	if b.labels == nil {
		b.labels = make(map[string]string)
	}
	b.labels[key] = value
	return b
}

func (b *TLSSecretBuilder) Annotation(key, value string) *TLSSecretBuilder {
	if b.annotations == nil {
		b.annotations = make(map[string]string)
	}
	b.annotations[key] = value
	return b
}

func (b *TLSSecretBuilder) CACertPEM(data []byte) *TLSSecretBuilder {
	b.cacertPEM = data
	return b
}

func (b *TLSSecretBuilder) EncodeCACert(data []byte) *TLSSecretBuilder {
	b.cacertPEM = certutil.EncodeCertPEM(data)
	return b
}

func (b *TLSSecretBuilder) CertPEM(data []byte) *TLSSecretBuilder {
	b.certPEM = data
	return b
}

func (b *TLSSecretBuilder) EncodeCert(data []byte) *TLSSecretBuilder {
	b.certPEM = certutil.EncodeCertPEM(data)
	return b
}

func (b *TLSSecretBuilder) KeyPEM(data []byte) *TLSSecretBuilder {
	b.keyPEM = data
	return b
}

func (b *TLSSecretBuilder) EncodeKey(data []byte) *TLSSecretBuilder {
	b.keyPEM = certutil.EncodePrivateKeyPEM(data)
	return b
}

func (b *TLSSecretBuilder) Build() corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        b.name,
			Namespace:   b.namespace,
			Labels:      b.labels,
			Annotations: b.annotations,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       b.certPEM,
			corev1.TLSPrivateKeyKey: b.keyPEM,
			KeyCACert:               b.cacertPEM,
			KeyIPSecSecretsFile:     []byte(": RSA tls.key\n"),
		},
	}
}
