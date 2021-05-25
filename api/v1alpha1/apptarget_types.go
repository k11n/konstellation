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
	"bytes"
	"fmt"

	"github.com/k11n/konstellation/pkg/utils/files"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

const (
	AppTargetHash = "k11n.dev/appTargetHash"
)

// AppTargetSpec defines a deployment target for App
type AppTargetSpec struct {
	App    string `json:"app"`
	Target string `json:"target"`
	Build  string `json:"build"`

	// +kubebuilder:validation:Required
	DeployMode DeployMode `json:"deployMode"`

	AppCommonSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	// +nullable
	// +optional
	Configs []string `json:"configs,omitempty"`

	// +kubebuilder:validation:Optional
	// +optional
	Scale ScaleSpec `json:"scale,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable
	// +optional
	Ingress *IngressConfig `json:"ingress,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable
	Prometheus *PrometheusSpec `json:"prometheus,omitempty"`
}

type AppTargetPhase string

var (
	AppTargetPhaseRunning   AppTargetPhase = "running"   // normal
	AppTargetPhaseDeploying AppTargetPhase = "deploying" // deploying a new release
	AppTargetPhaseHalted    AppTargetPhase = "halted"    // target halted
)

// AppTargetStatus defines the observed state of AppTarget
type AppTargetStatus struct {
	Phase           AppTargetPhase `json:"phase"`
	TargetRelease   string         `json:"targetRelease"`
	ActiveRelease   string         `json:"activeRelease"`
	DeployUpdatedAt metav1.Time    `json:"deployUpdatedAt"`
	// +kubebuilder:validation:Optional
	// +nullable
	LastScaledAt *metav1.Time `json:"lastScaledAt"`
	NumDesired   int32        `json:"numDesired"`
	NumReady     int32        `json:"numReady"`
	NumAvailable int32        `json:"numAvailable"`
	Hostname     string       `json:"hostname,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Build",type=string,JSONPath=`.spec.build`
// +kubebuilder:printcolumn:name="NumDesired",type=integer,JSONPath=`.status.numDesired`
// +kubebuilder:printcolumn:name="NumReady",type=integer,JSONPath=`.status.numReady`
// +kubebuilder:printcolumn:name="Min",type=integer,JSONPath=`.spec.scale.min`
// +kubebuilder:printcolumn:name="Max",type=integer,JSONPath=`.spec.scale.max`
// +kubebuilder:printcolumn:name="Hostname",type=string,JSONPath=`.status.hostname`

// AppTarget is the Schema for the apptargets API
type AppTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppTargetSpec   `json:"spec,omitempty"`
	Status AppTargetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AppTargetList contains a list of AppTarget
type AppTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppTarget `json:"items"`
}

/**
 * Namespace for all of resources that this obj owns
 */
func (at *AppTarget) ScopedName() string {
	return fmt.Sprintf("%s-%s", at.Spec.App, at.Spec.Target)
}

func (at *AppTarget) TargetNamespace() string {
	return at.Spec.Target
}

func (at *AppTarget) DesiredInstances() int32 {
	if at.Spec.DeployMode == DeployHalt {
		return 0
	}
	instances := at.Spec.Scale.Min
	if at.Status.NumDesired > instances {
		instances = at.Status.NumDesired
	}
	if instances > at.Spec.Scale.Max {
		instances = at.Spec.Scale.Max
	}
	return instances
}

func (at *AppTarget) NeedsService() bool {
	// TODO: allow local ports w/o creating a service
	return len(at.Spec.Ports) > 0
}

func (at *AppTarget) NeedsIngress() bool {
	if at.Spec.Ingress == nil {
		return false
	}
	return len(at.Spec.Ingress.Hosts) > 0
}

func (at *AppTarget) NeedsAutoscaler() bool {
	cpu := at.Spec.Resources.Requests.Cpu()
	if cpu == nil {
		return false
	}
	if at.Spec.Scale.TargetCPUUtilization == 0 {
		return false
	}
	if at.Spec.Scale.Min == at.Spec.Scale.Max {
		return false
	}
	return true
}

func (at *AppTarget) GetHash() string {
	return at.Labels[AppTargetHash]
}

// sets a label on the app target with its hash
func (at *AppTarget) UpdateHash() error {
	atCopy := at.DeepCopy()
	// clear fields that we don't need to include in hash
	atCopy.Spec.Ingress = nil
	atCopy.Status = AppTargetStatus{}
	atCopy.Labels = nil
	atCopy.Annotations = nil
	atCopy.Spec.DeployMode = DeployLatest
	atCopy.Spec.Scale = ScaleSpec{}
	encoder := json.NewSerializerWithOptions(json.DefaultMetaFactory, nil, nil,
		json.SerializerOptions{
			Yaml:   true,
			Pretty: true,
			Strict: false,
		})
	buf := bytes.NewBuffer(nil)
	err := encoder.Encode(atCopy, buf)
	if err != nil {
		return err
	}

	checksum, err := files.Sha1Checksum(buf)
	if err != nil {
		return err
	}

	at.Labels[AppTargetHash] = checksum

	return nil
}

func init() {
	SchemeBuilder.Register(&AppTarget{}, &AppTargetList{})
}
