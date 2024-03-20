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

package operator

import (
	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/about"
	"github.com/fabedge/fabedge/third_party/calicoapi"
)

var log = klogr.New().WithName("agent")

func init() {
	_ = apis.AddToScheme(scheme.Scheme)
	_ = calicoapi.AddToScheme(scheme.Scheme)
}

func Execute(opts *Options) error {
	defer klog.Flush()

	about.DisplayAndExitIfRequested()

	opts.ExtractAgentArgumentMap()

	if err := opts.Validate(); err != nil {
		log.Error(err, "invalid arguments found")
		return err
	}

	if err := opts.Complete(); err != nil {
		return err
	}

	return opts.RunManager()
}
