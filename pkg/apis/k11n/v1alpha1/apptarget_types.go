package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AppTargetSpec defines a deployment target for App
type AppTargetSpec struct {
	App    string `json:"app"`
	Target string `json:"target"`
	Build  string `json:"build"`

	// +kubebuilder:validation:Optional
	// +nullable
	// +optional
	Ports []PortSpec `json:"ports,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable
	// +optional
	Command []string `json:"command,omitempty"`
	// +kubebuilder:validation:Optional
	// +nullable
	// +optional
	Args []string `json:"args,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable
	// +optional
	Configs []string `json:"configs,omitempty"`
	// +kubebuilder:validation:Optional
	// +nullable
	// +optional
	Dependencies []AppReference `json:"dependencies,omitempty"`
	// +kubebuilder:validation:Optional
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// +kubebuilder:validation:Optional
	// +optional
	Scale ScaleSpec `json:"scale,omitempty"`
	// +kubebuilder:validation:Optional
	// +optional
	Probes ProbeConfig `json:"probes,omitempty"`
	// +kubebuilder:validation:Optional
	// +nullable
	// +optional
	Ingress *IngressConfig `json:"ingress,omitempty"`
}

// AppTargetStatus defines the observed state of AppTarget
type AppTargetStatus struct {
	TargetRelease   string      `json:"targetRelease"`
	ActiveRelease   string      `json:"activeRelease"`
	DeployUpdatedAt metav1.Time `json:"deployUpdatedAt"`
	// +kubebuilder:validation:Optional
	// +nullable
	LastScaledAt *metav1.Time `json:"lastScaledAt"`
	NumDesired   int32        `json:"numDesired"`
	NumReady     int32        `json:"numReady"`
	NumAvailable int32        `json:"numAvailable"`
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// +optional
	Ingress  string   `json:"ingress,omitempty"`
	Messages []string `json:"messages,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AppTarget is the Schema for the apptargets API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=apptargets,scope=Cluster
// +kubebuilder:printcolumn:name="NumDesired",type=integer,JSONPath=`.status.numDesired`
// +kubebuilder:printcolumn:name="NumReady",type=integer,JSONPath=`.status.numReady`
// +kubebuilder:printcolumn:name="NumAvailable",type=integer,JSONPath=`.status.numAvailable`
// +kubebuilder:printcolumn:name="Hostname",type=string,JSONPath=`.status.hostname`
type AppTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppTargetSpec   `json:"spec,omitempty"`
	Status AppTargetStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AppTargetList contains a list of AppTarget
type AppTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppTarget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AppTarget{}, &AppTargetList{})
}

/**
 * Namespace for all of resources that this obj owns
 */
func (at *AppTarget) ScopedName() string {
	return fmt.Sprintf("%s-%s", at.Spec.App, at.Spec.Target)
}

func (at *AppTarget) TargetNamespace() string {
	return at.ScopedName()
}

func (at *AppTarget) DesiredInstances() int32 {
	instances := at.Spec.Scale.Min
	if at.Status.NumDesired > instances {
		instances = at.Status.NumDesired
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
