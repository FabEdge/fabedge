package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const KeyNodeRoleConnector = "node-role.kubernetes.io/connector"

var _ Handler = &flannelNodeMocker{}

// flannelNodeMocker fill each edge node's annotations
// with flannel annotations of connector node
type flannelNodeMocker struct {
	client client.Client
	log    logr.Logger

	// the flannel annotations of a connector node
	flannelAnnotations map[string]string
	mux                sync.Mutex
}

func (handler *flannelNodeMocker) Do(ctx context.Context, node corev1.Node) error {
	flannelAnnotations, err := handler.getFlannelAnnotations(ctx)
	if err != nil {
		return err
	}

	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}

	changed := false
	for key, value := range flannelAnnotations {
		if node.Annotations[key] != value {
			node.Annotations[key] = value
			changed = true
		}
	}

	if !changed {
		return nil
	}

	if err = handler.client.Update(ctx, &node); err != nil {
		handler.log.Error(err, "failed to update node")
	}

	return err
}

func (handler *flannelNodeMocker) Undo(ctx context.Context, nodeName string) error {
	return nil
}

func (handler *flannelNodeMocker) getFlannelAnnotations(ctx context.Context) (map[string]string, error) {
	handler.mux.Lock()
	defer handler.mux.Unlock()

	if handler.flannelAnnotations == nil {
		var connectorNodes corev1.NodeList
		if err := handler.client.List(ctx, &connectorNodes, client.HasLabels{KeyNodeRoleConnector}); err != nil {
			return nil, err
		}

		if len(connectorNodes.Items) == 0 {
			return nil, fmt.Errorf("no connector node found")
		}

		// for now, we can assume only one connector exists and this connector
		// is running one specified connector node
		cn := connectorNodes.Items[0]
		handler.flannelAnnotations = map[string]string{}
		for key, value := range cn.Annotations {
			if strings.HasPrefix(key, "flannel") {
				handler.flannelAnnotations[key] = value
			}
		}
	}

	return handler.flannelAnnotations, nil
}
