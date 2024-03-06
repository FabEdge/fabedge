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
	"github.com/fabedge/fabedge/pkg/connector"
	logutil "github.com/fabedge/fabedge/pkg/util/log"
	flag "github.com/spf13/pflag"
	"time"
)

func main() {
	fs := flag.CommandLine
	cfg := &connector.Config{}

	logutil.AddFlags(fs)
	about.AddFlags(fs)
	cfg.AddFlags(fs)

	fs.StringVar(&cfg.CNIType, "cni-type", "flannel", "CNI type used in cloud")
	fs.DurationVar(&cfg.LeaderElection.LeaseDuration, "leader-lease-duration", 15*time.Second, "The duration that non-leader candidates will wait to force acquire leadership")
	fs.DurationVar(&cfg.LeaderElection.RenewDeadline, "leader-renew-deadline", 10*time.Second, "The duration that the acting controlplane will retry refreshing leadership before giving up")
	fs.DurationVar(&cfg.LeaderElection.RetryPeriod, "leader-retry-period", 2*time.Second, "The duration that the LeaderElector clients should wait between tries of actions")
	fs.StringSliceVar(&cfg.InitMembers, "connector-node-addresses", []string{}, "internal address of all connector nodes")
	fs.DurationVar(&cfg.SyncPeriod, "sync-period", 5*time.Minute, "period to sync routes/rules")
	fs.UintVar(&cfg.TunnelInitTimeout, "tunnel-init-timeout", 10, "The timeout of tunnel initiation. Unit: second")

	flag.Parse()

	connector.Execute(cfg)
}
