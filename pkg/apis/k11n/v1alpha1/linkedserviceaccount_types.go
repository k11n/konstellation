package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LinkedServiceAccountSpec defines the desired state of LinkedServiceAccount
type LinkedServiceAccountSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems:=1
	Policies []string `json:"policies"`
}

// ConnectedServiceAccountStatus defines the observed state of LinkedServiceAccount
type LinkedServiceAccountStatus struct {
	LinkedTargets []string `json:"linkedTargets"` // list of targets that are linked
	AWSRoleARN    string   `json:"awsRoleArn"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinkedServiceAccount is the Schema for the linkedserviceaccounts API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=linkedserviceaccounts,scope=Cluster
type LinkedServiceAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinkedServiceAccountSpec   `json:"spec,omitempty"`
	Status LinkedServiceAccountStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinkedServiceAccountList contains a list of ConnectedServiceAccount
type LinkedServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinkedServiceAccount `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinkedServiceAccount{}, &LinkedServiceAccountList{})
}
