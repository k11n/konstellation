package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RoleBasedServiceAccountSpec defines the desired state of RoleBasedServiceAccount
type RoleBasedServiceAccountSpec struct {
	ServiceAccount string   `json:"serviceAccount"`
	Target         string   `json:"target"`
	Policies       []string `json:"policies"`
}

// RoleBasedServiceAccountStatus defines the observed state of RoleBasedServiceAccount
type RoleBasedServiceAccountStatus struct {
	IAMRoleARN string `json:"iamRoleArn"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RoleBasedServiceAccount is the Schema for the rolebasedserviceaccounts API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=rolebasedserviceaccounts,scope=Cluster
type RoleBasedServiceAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RoleBasedServiceAccountSpec   `json:"spec,omitempty"`
	Status RoleBasedServiceAccountStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RoleBasedServiceAccountList contains a list of RoleBasedServiceAccount
type RoleBasedServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RoleBasedServiceAccount `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RoleBasedServiceAccount{}, &RoleBasedServiceAccountList{})
}
