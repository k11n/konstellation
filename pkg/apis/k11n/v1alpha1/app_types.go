package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AppSpec defines the desired state of App
type AppSpec struct {
	Ports      []PortSpec `json:"ports,omitempty"`
	DockerRepo string     `json:"docker_repo,omitempty"`
	Image      string     `json:"image"`

	// +optional
	ImageTag string `json:"image_tag,omitempty"`

	// +optional
	Command []string `json:"command,omitempty"`
	// +optional
	Args []string `json:"args,omitempty"`

	// defaults for the app, overridden by env
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// +optional
	Scale ScaleSpec `json:"scale,omitempty"`
	// +optional
	Probes ProbeConfig `json:"probes,omitempty"`
}

// AppStatus defines the observed state of App
type AppStatus struct {
	CurrentReplicas int      `json:"currentReplicas"`
	DesiredReplicas int      `json:"desiredReplicas"`
	Pods            []string `json:"pods,omitempty"`

	// +optional
	Hostname string `json:"hostname,omitempty"`
	// +optional
	Ingress string `json:"ingress,omitempty"`

	// TODO: this should be an enum type of some sort
	State string `json:"state"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// App is the Schema for the apps API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=apps,scope=Namespaced
type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppSpec   `json:"spec,omitempty"`
	Status AppStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AppList contains a list of App
type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App `json:"items"`
}

type PortSpec struct {
	Name string `json:"name"`
	Port int    `json:"port"`
	// +optional
	IngressPath string `json:"ingressPath,omitempty"`
}

type ResourceLimits struct {
	Memory string `json:"memory"`
	CPU    string `json:"cpu"`
}

type ScaleSpec struct {
	TargetCPUUtilization int `json:"targetCPUUtilizationPercentage,omitempty"`
	Min                  int `json:"min,omitempty"`
	Max                  int `json:"max,omitempty"`
	// +optional
	ScaleUp ScaleBehavior `json:"scaleUp,omitempty"`
	// +optional
	ScaleDown ScaleBehavior `json:"scaleDown,omitempty"`
}

type ScaleBehavior struct {
	Step  int `json:"step,omitempty"`
	Delay int `json:"delay,omitempty"`
}

type ProbeConfig struct {
	// +optional
	Liveness corev1.Probe `json:"liveness,omitempty"`
	// +optional
	Readiness corev1.Probe `json:"readiness,omitempty"`
}

type TargetConfig struct {
	Name string `json:"name"`

	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// +optional
	Scale ScaleSpec `json:"scale,omitempty"`
	// +optional
	Probes ProbeConfig `json:"probes,omitempty"`
}

func init() {
	SchemeBuilder.Register(&App{}, &AppList{})
}
