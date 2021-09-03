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

package community

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/jjeffery/stringset"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctlpkg "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apis "github.com/fabedge/fabedge/pkg/operator/apis/community/v1alpha1"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
)

const (
	controllerName = "community-controller"
)

type ObjectKey = client.ObjectKey

func AddToManager(config Config) error {
	mgr := config.Manager
	ctl, err := ctlpkg.New(
		controllerName,
		mgr,
		ctlpkg.Options{
			Reconciler: &communityController{
				store:  config.Store,
				client: mgr.GetClient(),
				log:    mgr.GetLogger().WithName(controllerName),
			},
		},
	)
	if err != nil {
		return err
	}

	return ctl.Watch(
		&source.Kind{Type: &apis.Community{}},
		&handler.EnqueueRequestForObject{},
	)
}

type Config struct {
	Manager manager.Manager
	Store   storepkg.Interface
}

type communityController struct {
	client client.Client
	log    logr.Logger
	store  storepkg.Interface
}

func (ctl *communityController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var community apis.Community
	if err := ctl.client.Get(ctx, request.NamespacedName, &community); err != nil {
		if errors.IsNotFound(err) {
			ctl.store.DeleteCommunity(request.Name)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if community.DeletionTimestamp != nil {
		ctl.store.DeleteCommunity(request.Name)
		return reconcile.Result{}, nil
	}

	ctl.store.SaveCommunity(types.Community{
		Name:    community.Name,
		Members: stringset.New(community.Spec.Members...),
	})
	return reconcile.Result{}, nil
}
