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

package agent

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/operator/types"
	certutil "github.com/fabedge/fabedge/pkg/util/cert"
	secretutil "github.com/fabedge/fabedge/pkg/util/secret"
)

var _ Handler = &certHandler{}

type certHandler struct {
	namespace string

	getEndpointName  types.GetNameFunc
	certManager      certutil.Manager
	certOrganization string

	client client.Client
	log    logr.Logger
}

func (handler *certHandler) Do(ctx context.Context, node corev1.Node) error {
	secretName := getCertSecretName(node.Name)

	log := handler.log.WithValues("nodeName", node.Name, "secretName", secretName, "namespace", handler.namespace)
	log.V(5).Info("Sync agent tls secret")

	var secret corev1.Secret
	err := handler.client.Get(ctx, ObjectKey{Name: secretName, Namespace: handler.namespace}, &secret)
	if err != nil {
		if !errors.IsNotFound(err) {
			handler.log.Error(err, "failed to get secret")
			return err
		}

		log.V(5).Info("TLS secret for agent is not found, generate it now")
		secret, err = handler.buildCertAndKeySecret(secretName, node)
		if err != nil {
			log.Error(err, "failed to create cert and key for agent")
			return err
		}

		err = handler.client.Create(ctx, &secret)
		if err != nil {
			log.Error(err, "failed to create secret")
			return err
		}

		return errRestartAgent
	}

	certPEM := secretutil.GetCert(secret)
	err = handler.certManager.VerifyCertInPEM(certPEM, certutil.ExtKeyUsagesServerAndClient)
	if err == nil {
		log.V(5).Info("cert is verified")
		return nil
	}

	log.Error(err, "failed to verify cert, need to regenerate a cert to agent")
	secret, err = handler.buildCertAndKeySecret(secretName, node)
	if err != nil {
		log.Error(err, "failed to recreate cert and key for agent")
		return err
	}

	if err = handler.client.Update(ctx, &secret); err != nil {
		log.Error(err, "failed to save secret")
		return err
	}

	return errRestartAgent
}

func (handler *certHandler) buildCertAndKeySecret(secretName string, node corev1.Node) (corev1.Secret, error) {
	keyDER, csr, err := certutil.NewCertRequest(certutil.Request{
		CommonName:   handler.getEndpointName(node.Name),
		Organization: []string{handler.certOrganization},
	})
	if err != nil {
		return corev1.Secret{}, err
	}

	certDER, err := handler.certManager.SignCert(csr)
	if err != nil {
		return corev1.Secret{}, err
	}

	return secretutil.TLSSecret().
		Name(secretName).
		Namespace(handler.namespace).
		EncodeCert(certDER).
		EncodeKey(keyDER).
		CACertPEM(handler.certManager.GetCACertPEM()).
		Label(constants.KeyCreatedBy, constants.AppOperator).
		Label(constants.KeyNode, node.Name).Build(), nil
}

func (handler *certHandler) Undo(ctx context.Context, nodeName string) error {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getCertSecretName(nodeName),
			Namespace: handler.namespace,
		},
	}
	err := handler.client.Delete(ctx, &secret)
	if err != nil {
		if errors.IsNotFound(err) {
			err = nil
		} else {
			handler.log.Error(err, "failed to delete secret", "name", secret.Name, "namespace", secret.Namespace)
		}
	}
	return err
}

func getCertSecretName(nodeName string) string {
	return fmt.Sprintf("fabedge-agent-tls-%s", nodeName)
}
