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

package constants

const (
	KeyPodSubnets          = "fabedge.io/subnets"
	KeyFabEdgeAPP          = "fabedge.io/app"
	KeyFabEdgeName         = "fabedge.io/name"
	KeyCreatedBy           = "fabedge.io/created-by"
	KeyNode                = "fabedge.io/node"
	KeyCluster             = "fabedge.io/cluster"
	KeyNodePublicAddresses = "fabedge.io/node-public-addresses"
	KeyPodHash             = "fabedge.io/pod-spec-hash"
	AppAgent               = "fabedge-agent"
	AppOperator            = "fabedge-operator"

	ConnectorConfigFileName = "tunnels.yaml"
	ConnectorConfigName     = "connector-config"
	ConnectorTLSName        = "connector-tls"
)

const (
	CNIFlannel = "flannel"
	CNICalico  = "calico"
)

const (
	TableStrongswan = 220

	DefaultMediatorName = "mediator"
)
