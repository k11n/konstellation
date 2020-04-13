package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CertificateRefSpec defines the desired state of CertificateRef
type CertificateRefSpec struct {
	ProviderID         string      `json:"providerId"`
	Domain             string      `json:"domain"`
	Issuer             string      `json:"issuer"`
	Status             string      `json:"status"`
	ExpiresAt          metav1.Time `json:"expiresAt"`
	KeyAlgorithm       string      `json:"keyAlgorithm"`
	SignatureAlgorithm string      `json:"signatureAlgorithm"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CertificateRef is the Schema for the certificates API
// +kubebuilder:resource:path=certificaterefs,scope=Cluster
type CertificateRef struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CertificateRefSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CertificateRefList contains a list of CertificateRef
type CertificateRefList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CertificateRef `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CertificateRef{}, &CertificateRefList{})
}
