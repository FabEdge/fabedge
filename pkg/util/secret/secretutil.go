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
