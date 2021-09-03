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
	"flag"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// WrapReconcile returns a reconcile.Reconcile implementation that delegates to inner and
// writes the request to requests after Reconcile is finished.
func WrapReconcile(inner reconcile.Reconciler) (reconcile.Reconciler, chan reconcile.Request) {
	requests := make(chan reconcile.Request)
	fn := reconcile.Func(func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
		result, err := inner.Reconcile(ctx, req)
		requests <- req
		return result, err
	})
	return fn, requests
}

// WrapReconcileFunc returns a reconcile.Func implementation that delegates to inner and
// writes the request to requests after Reconcile is finished.
func WrapReconcileFunc(inner reconcile.Func) (reconcile.Func, chan reconcile.Request) {
	requests := make(chan reconcile.Request)
	fn := func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
		result, err := inner.Reconcile(ctx, req)
		requests <- req
		return result, err
	}
	return fn, requests
}

// DrainChan drains the request chan time for drainTimeout
func DrainChan(requests <-chan reconcile.Request, timeout time.Duration) {
	for {
		select {
		case <-requests:
			continue
		case <-time.After(timeout):
			return
		}
	}
}

func SetupLogger() {
	level, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		level = "-1"
	}
	klog.InitFlags(nil)
	_ = flag.Set("v", level)
	logf.SetLogger(klogr.New().V(5))
}

func StartTestEnv() (env *envtest.Environment, cfg *rest.Config, cli client.Client, err error) {
	return StartTestEnvWithCRDAndScheme([]string{}, scheme.Scheme)
}

func StartTestEnvWithCRD(CRDDirectoryPaths []string) (env *envtest.Environment, cfg *rest.Config, cli client.Client, err error) {
	return StartTestEnvWithCRDAndScheme(CRDDirectoryPaths, scheme.Scheme)
}

func StartTestEnvWithCRDAndScheme(CRDDirectoryPaths []string, scheme *runtime.Scheme) (env *envtest.Environment, cfg *rest.Config, cli client.Client, err error) {
	env = &envtest.Environment{
		CRDDirectoryPaths: CRDDirectoryPaths,
	}

	cfg, err = env.Start()
	if err != nil {
		return
	}

	if cfg == nil {
		err = fmt.Errorf("no rest config created")
		return
	}

	// +kubebuilder:scaffold:scheme
	cli, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return
	}

	if cli == nil {
		err = fmt.Errorf("no k8s client created")
	}
	return
}

func GenerateGetNameFunc(namePrefix string) func() string {
	var index = 0
	return func() string {
		for index < 10000 {
			name := fmt.Sprintf("%s-%d", namePrefix, index)
			index++
			index %= 10000

			return name
		}

		return ""
	}
}
