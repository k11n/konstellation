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

// NodepoolSpec defines the desired state of Nodepool
type NodepoolSpec struct {
	Autoscale   bool             `json:"autoscale" desc:"Uses autoscale"`
	MinSize     int64            `json:"minSize" desc:"Min number of nodes"`
	MaxSize     int64            `json:"maxSize" desc:"Max number of nodes"`
	MachineType string           `json:"machineType" desc:"Machine type"`
	DiskSizeGiB int              `json:"diskSizeGiB" desc:"Disk size (GiB)"`
	RequiresGPU bool             `json:"requiresGPU" desc:"Needs GPU"`
	AWS         *AWSNodepoolSpec `json:"aws,omitempty"`
}

// NodepoolStatus defines the observed state of Nodepool
type NodepoolStatus struct {
	// +kubebuilder:validation:Optional
	// +nullable
	Nodes    []string           `json:"nodes"`
	NumReady int                `json:"numReady"`
	AWS      *AWSNodepoolStatus `json:"aws,omitempty"`
}

type AWSNodepoolSpec struct {
	AMIType             string `json:"amiType" desc:"AMI Type"`
	SSHKeypair          string `json:"sshKeypair" desc:"SSH keypair"`
	ConnectFromAnywhere bool   `json:"connectFromAnywhere" desc:"Allow connection from internet"`
}

type AWSNodepoolStatus struct {
	// set only after nodepool is created
	// +kubebuilder:validation:Optional
	ASGID string `json:"asgId,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="AutoScale",type=boolean,JSONPath=`.spec.autoscale`
// +kubebuilder:printcolumn:name="MachineType",type=string,JSONPath=`.spec.machineType`
// +kubebuilder:printcolumn:name="MinSize",type=integer,JSONPath=`.spec.minSize`
// +kubebuilder:printcolumn:name="MaxSize",type=integer,JSONPath=`.spec.maxSize`
// +kubebuilder:printcolumn:name="NumReady",type=string,JSONPath=`.status.numReady`

// Nodepool is the Schema for the nodepools API
type Nodepool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodepoolSpec   `json:"spec,omitempty"`
	Status NodepoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NodepoolList contains a list of Nodepool
type NodepoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Nodepool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Nodepool{}, &NodepoolList{})
}
