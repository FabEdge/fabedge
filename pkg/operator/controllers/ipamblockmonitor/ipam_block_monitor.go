package ipamblockmonitor

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	handlerpkg "sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/fabedge/fabedge/pkg/operator/types"
	"github.com/fabedge/fabedge/third_party/calicoapi"
)

const (
	controllerName = "ipam-block-monitor"
)

type Config struct {
	Store   types.PodCIDRStore
	Manager manager.Manager
}

func AddToManager(cfg Config) error {
	mgr := cfg.Manager

	c, err := controller.New(
		controllerName,
		mgr,
		controller.Options{
			Reconciler: &ipamBlockMonitor{
				Config: cfg,
				log:    mgr.GetLogger().WithName(controllerName),
				client: mgr.GetClient(),
			},
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: &calicoapi.IPAMBlock{}},
		&handlerpkg.EnqueueRequestForObject{},
	)
}

type ipamBlockMonitor struct {
	Config

	client client.Client
	log    logr.Logger
}

func (m *ipamBlockMonitor) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := m.log.WithValues("request", request)

	var block calicoapi.IPAMBlock
	if err := m.client.Get(ctx, request.NamespacedName, &block); err != nil {
		log.Error(err, "failed to get IPAMBlock")

		if errors.IsNotFound(err) {
			cidr, err := ParseIPAMBlockName(request.Name)
			if err != nil {
				log.Error(err, "failed to parse ipam block name")
				return reconcile.Result{}, nil
			}

			m.Store.RemoveByPodCIDR(cidr)
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	if block.DeletionTimestamp != nil || block.Spec.Deleted {
		log.Info("IPAM block is deleted, remove CIDR from store")
		m.Store.RemoveByPodCIDR(block.Spec.CIDR)
		return reconcile.Result{}, nil
	}

	cidr, nodeName := block.Spec.CIDR, GetNodeName(block)
	if nodeName != "" {
		m.Store.Append(nodeName, cidr)
	} else {
		log.Info("Affinity is not recognizable, remove CIDR from store", "nodeName", nodeName)
		m.Store.RemoveByPodCIDR(block.Spec.CIDR)
	}

	return reconcile.Result{}, nil
}

// ParseIPAMBlockName convert ipam block name into pod cidr
// IPv4 only
func ParseIPAMBlockName(name string) (string, error) {
	digits := strings.Split(name, "-")
	if len(digits) != 5 {
		return "", fmt.Errorf("not a IPv4 ipam block name: %s", name)
	}

	return fmt.Sprintf("%s.%s.%s.%s/%s", digits[0], digits[1], digits[2], digits[3], digits[4]), nil
}

func GetNodeName(block calicoapi.IPAMBlock) string {
	if block.Spec.Affinity == nil {
		return ""
	}

	if strings.HasPrefix(*block.Spec.Affinity, "host:") {
		return (*block.Spec.Affinity)[5:]
	}

	return ""
}
