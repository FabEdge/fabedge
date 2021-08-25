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
