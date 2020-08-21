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

// IngressRequestSpec defines the desired state of IngressRequest
type IngressRequestSpec struct {
	Hosts []string `json:"hosts"`

	// +optional
	Paths []string `json:"paths,omitempty"`

	// +optional
	RequireHTTPS bool `json:"requireHttps,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// IngressRequestStatus defines the observed state of IngressRequest
type IngressRequestStatus struct {
	Address string `json:"address"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// IngressRequest is the Schema for the ingressrequests API
type IngressRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IngressRequestSpec   `json:"spec,omitempty"`
	Status IngressRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IngressRequestList contains a list of IngressRequest
type IngressRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IngressRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IngressRequest{}, &IngressRequestList{})
}
