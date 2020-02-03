package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AppTargetSpec defines the desired state of AppTarget
type AppTargetSpec struct {
	App    string `json:"app"`
	Target string `json:"target"`
	Build  string `json:"build"`

	// +optional
	Ports []PortSpec `json:"ports,omitempty"`

	// +optional
	Command []string `json:"command,omitempty"`
	// +optional
	Args []string `json:"args,omitempty"`

	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// +optional
	Scale ScaleSpec `json:"scale,omitempty"`
	// +optional
	Probes ProbeConfig `json:"probes,omitempty"`
	// +optional
	IngressHosts []string `json:"ingressHosts,omitempty"`
}

// AppTargetStatus defines the observed state of AppTarget
type AppTargetStatus struct {
	CurrentReplicas     int32        `json:"currentReplicas"`
	DesiredReplicas     int32        `json:"desiredReplicas"`
	UnavailableReplicas int32        `json:"unavailableReplicas"`
	Pods                []string     `json:"pods,omitempty"`
	LastScaleTime       *metav1.Time `json:"lastScaleTime,omitempty"`

	// +optional
	Hostname string `json:"hostname,omitempty"`
	// +optional
	Ingress string `json:"ingress,omitempty"`

	// TODO: this should be an enum type of some sort
	State string `json:"state"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AppTarget is the Schema for the apptargets API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=apptargets,scope=Cluster
// +kubebuilder:printcolumn:name="CurrentReplicas",type=integer,JSONPath=`.status.currentReplicas`
// +kubebuilder:printcolumn:name="DesiredReplicas",type=integer,JSONPath=`.status.desiredReplicas`
// +kubebuilder:printcolumn:name="Hostname",type=string,JSONPath=`.status.hostname`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
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

func (a *AppTargetSpec) ContainerPorts() []corev1.ContainerPort {
	ports := []corev1.ContainerPort{}
	for _, p := range a.Ports {
		ports = append(ports, corev1.ContainerPort{
			Name:          p.Name,
			ContainerPort: p.Port,
			Protocol:      p.Protocol,
		})
	}
	return ports
}
