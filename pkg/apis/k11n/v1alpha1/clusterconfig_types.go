package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterComponent struct {
	ComponentSpec `json:",inline"`
	// +kubebuilder:validation:Optional
	// +nullable
	Config map[string]string `json:"config"`
}

type ComponentSpec struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClusterConfigSpec defines the desired state of ClusterConfig
type ClusterConfigSpec struct {
	Version string `json:"version"`
	Cloud   string `json:"cloud"`
	// +kubebuilder:validation:Optional
	// +nullable
	AWSConfig *AWSCloudConfig `json:"awsConfig"`
	// +kubebuilder:validation:Optional
	// +nullable
	Targets    []string           `json:"targets"`
	Components []ClusterComponent `json:"components"`
}

// ClusterConfigStatus defines the observed state of ClusterConfig
type ClusterConfigStatus struct {
	InstalledComponents []ComponentSpec `json:"components"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterConfig is the Schema for the clusterconfigs API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=clusterconfigs,scope=Cluster
type ClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterConfigSpec   `json:"spec,omitempty"`
	Status ClusterConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterConfigList contains a list of ClusterConfig
type ClusterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterConfig `json:"items"`
}

type AWSCloudConfig struct {
	Region         string       `json:"region"`
	Vpc            string       `json:"vpc"`
	SecurityGroups []string     `json:"securityGroups"`
	PrivateSubnets []*AWSSubnet `json:"privateSubnets"`
	PublicSubnets  []*AWSSubnet `json:"publicSubnets"`
	AlbRoleArn     string       `json:"albRoleArn"`
}

type AWSSubnet struct {
	SubnetId         string `json:"subnetId"`
	IsPublic         bool   `json:"isPublic"`
	Ipv4Cidr         string `json:"ipv4Cidr"`
	AvailabilityZone string `json:"availabilityZone"`
}

func init() {
	SchemeBuilder.Register(&ClusterConfig{}, &ClusterConfigList{})
}
