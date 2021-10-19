package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
import "github.com/fabedge/fabedge/pkg/common/netconf"

type TunnelEndpoint struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	// Public addresses can be IP, DNS
	PublicAddresses []string `json:"publicAddresses,omitempty"`
	// PodCIDRs, ServiceCIDR
	Subnets []string `json:"subnets,omitempty"`
	// Internal IPs of kubernetes node
	NodeSubnets []string `json:"nodeSubnets,omitempty"`
	// Type of endpoints: Connector or EdgeNode
	Type netconf.EndpointType `json:"type,omitempty"`
}

type ClusterSpec struct {
	// The name of a cluster
	Name string `json:"name,omitempty"`
	// Token is used by child cluster to access root cluster's apiserver
	Token string `json:"token,omitempty"`
	// Endpoints of connector and exported edge nodes of a cluster
	EndPoints []TunnelEndpoint `json:"endPoints,omitempty"`
}

// Cluster is used to represent a cluster's endpoints of connector and edge nodes
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Name",type="string",JSONPath=".spec.name",description="The name of the cluster"
// +kubebuilder:printcolumn:name="Token",type="string",JSONPath=".spec.token",description="The token used to connect root cluster"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="How long a community is created"
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterSpec `json:"spec,omitempty"`
}

// ClusterList contains a list of clusters
// +kubebuilder:object:root=true
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}
