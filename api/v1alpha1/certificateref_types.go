/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// CertificateRef is the Schema for the certificaterefs API
type CertificateRef struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CertificateRefSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// CertificateRefList contains a list of CertificateRef
type CertificateRefList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CertificateRef `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CertificateRef{}, &CertificateRefList{})
}
