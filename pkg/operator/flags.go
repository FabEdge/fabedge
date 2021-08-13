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

package operator

import "flag"

var (
	version bool

	namespace   string
	edgePodCIDR string

	agentImage       string
	strongswanImage  string
	endpointIDFormat string
	masqOutgoing     bool

	connectorConfig             string
	connectorConfigSyncInterval int64

	leaderElection      bool
	leaderElectionID    string
	leaderLeaseDuration int64
	leaderRenewDeadline int64

	ipvsScheduler string
	useXfrm       bool
)

func init() {
	flag.BoolVar(&version, "version", false, "display version info")

	flag.StringVar(&namespace, "namespace", "kube-system", "The namespace in which agent pod and configmaps will be created")
	flag.StringVar(&edgePodCIDR, "edge-pod-cidr", "2.2.0.0/16", "Specify range of IP addresses for the edge pod. If set, fabedge-operator will automatically allocate CIDRs for every edge node")

	flag.StringVar(&agentImage, "agent-image", "fabedge/agent:latest", "Used the image to create agent container of agent pod")
	flag.StringVar(&strongswanImage, "strongswan-image", "strongswan:5.9.1", "Used the image to create strongswan container of agent pod")
	flag.StringVar(&endpointIDFormat, "endpoint-id-format", "C=CN, O=StrongSwan, CN={node}", "the id format of tunnel endpoint")
	flag.BoolVar(&masqOutgoing, "masq-outgoing", true, "Determine if perform outbound NAT from edge pods to outside of the cluster")

	flag.StringVar(&connectorConfig, "connector-config", "cloud-tunnels-config", "the name of configmap for fabedge-connector")
	flag.Int64Var(&connectorConfigSyncInterval, "connector-config-sync-interval", 5, "the interval(seconds) to synchronize connector configmap")

	flag.BoolVar(&leaderElection, "leader-election", false, "Determines whether or not to use leader election")
	flag.StringVar(&leaderElectionID, "leader-election-id", "fabedge-operator-leader", "The name of the resource that leader election will use for holding the leader lock")
	flag.Int64Var(&leaderLeaseDuration, "leader-lease-duration", 15, "The duration(seconds) that non-leader candidates will wait to force acquire leadership")
	flag.Int64Var(&leaderRenewDeadline, "leader-renew-deadline", 4, "The duration(seconds) that the acting controlplane will retry refreshing leadership before giving up")

	flag.StringVar(&ipvsScheduler, "ipvs-scheduler", "rr", "The ipvs scheduler for each service")
	flag.BoolVar(&useXfrm, "use-xfrm", false, "let agent use xfrm if edge OS supports")
}
