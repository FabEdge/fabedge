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

package test

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PurgeAllSecrets(cli client.Client, opts ...client.ListOption) error {
	var secrets corev1.SecretList
	err := cli.List(context.TODO(), &secrets)
	if err != nil {
		return err
	}

	for _, secret := range secrets.Items {
		if err := cli.Delete(context.TODO(), &secret); err != nil {
			return err
		}
	}

	return nil
}

func PurgeAllConfigMaps(cli client.Client, opts ...client.ListOption) error {
	var configs corev1.ConfigMapList
	err := cli.List(context.TODO(), &configs)
	if err != nil {
		return err
	}

	for _, secret := range configs.Items {
		if err := cli.Delete(context.TODO(), &secret); err != nil {
			return err
		}
	}

	return nil
}

func PurgeAllPods(cli client.Client, opts ...client.ListOption) error {
	var pods corev1.PodList
	err := cli.List(context.TODO(), &pods)
	if err != nil {
		return err
	}

	for _, secret := range pods.Items {
		if err := cli.Delete(context.TODO(), &secret); err != nil {
			return err
		}
	}

	return nil
}

func PurgeAllNodes(cli client.Client, opts ...client.ListOption) error {
	var nodes corev1.NodeList
	err := cli.List(context.TODO(), &nodes)
	if err != nil {
		return err
	}

	for _, secret := range nodes.Items {
		if err := cli.Delete(context.TODO(), &secret); err != nil {
			return err
		}
	}

	return nil
}
