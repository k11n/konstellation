package v1alpha1

import (
	"fmt"

	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AppSpec defines the desired state of App
type AppSpec struct {
	DockerRepo string `json:"docker_repo,omitempty"`
	Image      string `json:"image"`

	// +optional
	Ports []PortSpec `json:"ports,omitempty"`

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

	//+kubebuilder:validation:MinItems:=1
	Targets []TargetConfig `json:"targets"`
}

// AppStatus defines the observed state of App
type AppStatus struct {
	ActiveTargets []string `json:"activeTargets"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// App is the Schema for the apps API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=apps,scope=Cluster
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

	// the host to match for the ingress,
	// +optional
	IngressHosts []string `json:"ingressHosts,omitempty"`
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

func (a *AppSpec) ScaleSpecForTarget(target string) *ScaleSpec {
	scale := a.Scale.DeepCopy()
	tc := a.GetTargetConfig(target)

	if tc != nil {
		mergo.Merge(scale, &tc.Scale)
	}
	if scale.Min == 0 {
		scale.Min = 1
	}
	if scale.Max == 0 {
		scale.Max = scale.Min
	}
	return scale
}

func (a *AppSpec) EnvForTarget(target string) []corev1.EnvVar {
	tc := a.GetTargetConfig(target)
	env := []corev1.EnvVar{}
	seen := map[string]bool{}
	if tc != nil {
		for _, ev := range tc.Env {
			seen[ev.Name] = true
			env = append(env, ev)
		}
	}

	for _, ev := range a.Env {
		if !seen[ev.Name] {
			env = append(env, ev)
		}
	}
	return env
}

func (a *AppSpec) ResourcesForTarget(target string) *corev1.ResourceRequirements {
	res := a.Resources.DeepCopy()
	tc := a.GetTargetConfig(target)
	if tc != nil {
		mergo.Merge(res, &tc.Resources)
	}
	return res
}

func (a *AppSpec) ProbesForTarget(target string) *ProbeConfig {
	probes := a.Probes.DeepCopy()
	tc := a.GetTargetConfig(target)
	if tc != nil {
		mergo.Merge(probes, &tc.Probes)
	}
	return probes
}

func (a *AppSpec) GetTargetConfig(target string) *TargetConfig {
	var targetConf *TargetConfig
	for i, _ := range a.Targets {
		if a.Targets[i].Name == target {
			targetConf = &a.Targets[i]
			break
		}
	}
	return targetConf
}

func (a *App) GetAppTargetName(target string) string {
	return fmt.Sprintf("%s-%s", a.Name, target)
}
