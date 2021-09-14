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

import corev1 "k8s.io/api/core/v1"

// GetCACert get the ca cert from the secret by the key ca.crt
func GetCACert(secret corev1.Secret) []byte {
	return secret.Data[KeyCACert]
}

// GetCAKey get the ca cert from the secret by the key ca.key
func GetCAKey(secret corev1.Secret) []byte {
	return secret.Data[KeyCAKey]
}

// GetCA get the ca cert/key from the secret by the key ca.crt and ca.key
func GetCA(secret corev1.Secret) ([]byte, []byte) {
	return secret.Data[KeyCACert], secret.Data[KeyCAKey]
}

// GetCert get the cert from the secret by the key tls.crt
func GetCert(secret corev1.Secret) []byte {
	return secret.Data[corev1.TLSCertKey]
}

// GetCertAndKey get the cert and Key from the secret by the key tls.crt and tls.key
func GetCertAndKey(secret corev1.Secret) ([]byte, []byte) {
	return secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey]
}
