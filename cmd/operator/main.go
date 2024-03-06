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

package main

import (
	"github.com/fabedge/fabedge/pkg/common/about"
	logutil "github.com/fabedge/fabedge/pkg/util/log"
	flag "github.com/spf13/pflag"
	"os"
	"time"

	"github.com/fabedge/fabedge/pkg/operator"
)

func main() {
	opts := &operator.Options{}

	fs := flag.CommandLine
	logutil.AddFlags(fs)
	about.AddFlags(fs)
	opts.AddFlags(fs)

	flag.StringVar(&opts.CNIType, "cni-type", "", "The CNI name in your kubernetes cluster")
	opts.ManagerOpts.LeaseDuration = flag.Duration("leader-lease-duration", 15*time.Second, "The duration that non-leader candidates will wait to force acquire leadership")
	opts.ManagerOpts.RenewDeadline = flag.Duration("leader-renew-deadline", 10*time.Second, "The duration that the acting controlplane will retry refreshing leadership before giving up")
	opts.ManagerOpts.RetryPeriod = flag.Duration("leader-retry-period", 2*time.Second, "The duration that the LeaderElector clients should wait between tries of actions")

	flag.Parse()

	if err := operator.Execute(opts); err != nil {
		os.Exit(1)
	}
}
