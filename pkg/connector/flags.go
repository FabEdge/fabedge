// Copyright 2021 BoCloud
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

package connector

import (
	"flag"
	"time"
)

var (
	tunnelConfig     string
	certFile         string
	viciSocket       string
	syncPeriod       time.Duration
	debounceDuration time.Duration
	cniType          string
)

func init() {
	flag.StringVar(&tunnelConfig, "tunnel-config", "/etc/fabedge/tunnels.yaml", "tunnel config file")
	flag.StringVar(&certFile, "cert-file", "/etc/ipsec.d/certs/tls.crt", "TLS certificate file")
	flag.StringVar(&viciSocket, "vici-socket", "/var/run/charon.vici", "vici socket file")
	flag.StringVar(&cniType, "cni-type", "CALICO", "CNI type used in cloud")
	flag.DurationVar(&syncPeriod, "sync-period", time.Minute*5, "period to sync routes/rules")
	flag.DurationVar(&debounceDuration, "debounce-duration", time.Second*5, "period to sync routes/rules")
}
