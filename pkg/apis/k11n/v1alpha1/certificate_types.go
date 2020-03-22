package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CertificateSpec defines the desired state of Certificate
type CertificateSpec struct {
	ProviderID         string      `json:"providerId"`
	Domain             string      `json:"domain"`
	Issuer             string      `json:"issuer"`
	Status             string      `json:"status"`
	ExpiresAt          metav1.Time `json:"expiresAt"`
	KeyAlgorithm       string      `json:"keyAlgorithm"`
	SignatureAlgorithm string      `json:"signatureAlgorithm"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Certificate is the Schema for the certificates API
// +kubebuilder:resource:path=certificates,scope=Namespaced
type Certificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CertificateSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CertificateList contains a list of Certificate
type CertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Certificate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Certificate{}, &CertificateList{})
}
