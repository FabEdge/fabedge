package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EndpointType string

const (
	Connector EndpointType = "Connector"
	EdgeNode  EndpointType = "EdgeNode"
)

type Endpoint struct {
	ID   string `yaml:"id,omitempty" json:"id,omitempty"`
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// public addresses can be IP, DNS
	PublicAddresses []string `yaml:"publicAddresses,omitempty" json:"publicAddresses,omitempty"`
	// pod subnets
	Subnets []string `yaml:"subnets,omitempty" json:"subnets,omitempty"`
	// internal IPs of kubernetes node
	NodeSubnets []string     `yaml:"nodeSubnets,omitempty" json:"nodeSubnets,omitempty"`
	Type        EndpointType `yaml:"type,omitempty" json:"type,omitempty"`
}

type ClusterSpec struct {
	// Token is used by child cluster to access root cluster's apiserver
	Token string `json:"token,omitempty"`
	// Endpoints of connector and exported edge nodes of a cluster
	EndPoints []Endpoint `json:"endPoints,omitempty"`
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
