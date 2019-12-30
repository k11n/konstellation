package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodepoolSpec defines the desired state of Nodepool
type NodepoolSpec struct {
	Autoscale   bool         `json:"autoscale" desc:"Uses autoscale"`
	MinSize     int64        `json:"minSize" desc:"Min number of nodes"`
	MaxSize     int64        `json:"maxSize" desc:"Max number of nodes"`
	MachineType string       `json:"machineType" desc:"Machine type"`
	DiskSizeGiB int          `json:"diskSizeGiB" desc:"Disk size (GiB)"`
	RequiresGPU bool         `json:"requiresGPU" desc:"Needs GPU"`
	AWS         *NodePoolAWS `json:"aws,omitempty"`
}

type NodePoolAWS struct {
	RoleARN             string `json:"roleArn" desc:"Node role"`
	AMIType             string `json:"amiType"`
	SSHKeypair          string `json:"sshKeypair" desc:"SSH keypair"`
	ConnectFromAnywhere bool   `json:"connectFromAnywhere" desc:"Allow connection from internet"`
	SecurityGroupId     string `json:"securityGroupId,omitempty"`
	SecurityGroupName   string `json:"securityGroupName,omitempty" desc:"Security group (for connection)"`
}

// NodepoolStatus defines the observed state of Nodepool
type NodepoolStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Nodepool is the Schema for the nodepools API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=nodepools,scope=Namespaced
type Nodepool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodepoolSpec   `json:"spec,omitempty"`
	Status NodepoolStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodepoolList contains a list of Nodepool
type NodepoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Nodepool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Nodepool{}, &NodepoolList{})
}
