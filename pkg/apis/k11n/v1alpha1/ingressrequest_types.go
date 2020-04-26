package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressRequestSpec defines the desired state of IngressRequest
type IngressRequestSpec struct {
	Hosts []string `json:"hosts"`
	Port  int32    `json:"port,omitempty"`
}

// IngressRequestStatus defines the observed state of IngressRequest
type IngressRequestStatus struct {
	Address string `json:"address"`
	Message string `json:"message"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IngressRequest is the Schema for the ingressrequests API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ingressrequests,scope=Cluster
type IngressRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IngressRequestSpec   `json:"spec,omitempty"`
	Status IngressRequestStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IngressRequestList contains a list of IngressRequest
type IngressRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IngressRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IngressRequest{}, &IngressRequestList{})
}
