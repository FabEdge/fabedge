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
	flag "github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/about"
	apis "github.com/fabedge/fabedge/pkg/operator/apis/community/v1alpha1"
	logutil "github.com/fabedge/fabedge/pkg/util/log"
	"github.com/fabedge/fabedge/third_party/calicoapi"
)

var log = klogr.New().WithName("agent")

func init() {
	_ = apis.AddToScheme(scheme.Scheme)
	_ = calicoapi.AddToScheme(scheme.Scheme)
}

func Execute() error {
	defer klog.Flush()

	opts := &Options{}

	fs := flag.CommandLine
	logutil.AddFlags(fs)
	about.AddFlags(fs)
	opts.AddFlags(fs)

	flag.Parse()

	about.DisplayAndExitIfRequested()

	if err := opts.Complete(); err != nil {
		return err
	}

	if err := opts.Validate(); err != nil {
		log.Error(err, "invalid arguments found")
		return err
	}

	return opts.RunManager()
}
