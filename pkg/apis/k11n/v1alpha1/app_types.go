package v1alpha1

import (
	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	Port int32  `json:"port"`
	// +optional
	Protocol corev1.Protocol `json:"protocol,omitempty"`
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
	Liveness *Probe `json:"liveness,omitempty"`
	// +optional
	Readiness *Probe `json:"readiness,omitempty"`
	// +optional
	Startup *Probe `json:"startup,omitempty"`
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
	return a.Name
}

func (p *Probe) ToCoreProbe() *corev1.Probe {
	coreHander := corev1.Handler{
		Exec: p.Handler.Exec,
	}
	if p.Handler.HTTPGet != nil {
		hg := p.Handler.HTTPGet
		coreHander.HTTPGet = &corev1.HTTPGetAction{
			Path:        hg.Path,
			Host:        hg.Host,
			Port:        intstr.FromInt(hg.Port),
			Scheme:      hg.Scheme,
			HTTPHeaders: hg.HTTPHeaders,
		}
	}
	coreP := corev1.Probe{
		Handler:             coreHander,
		InitialDelaySeconds: p.InitialDelaySeconds,
		TimeoutSeconds:      p.TimeoutSeconds,
		PeriodSeconds:       p.PeriodSeconds,
		SuccessThreshold:    p.SuccessThreshold,
		FailureThreshold:    p.FailureThreshold,
	}
	return &coreP
}

// ---------------------------------------------------------------------------//
// a duplication of core Kube types, repeated here to avoid dependency on intOrString type
// Probe describes a health check to be performed against a container to determine whether it is
// alive or ready to receive traffic.
type Probe struct {
	// The action taken to determine the health of a container
	Handler `json:",inline" protobuf:"bytes,1,opt,name=handler"`
	// Number of seconds after the container has started before liveness probes are initiated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty" protobuf:"varint,2,opt,name=initialDelaySeconds"`
	// Number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty" protobuf:"varint,3,opt,name=timeoutSeconds"`
	// How often (in seconds) to perform the probe.
	// Default to 10 seconds. Minimum value is 1.
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty" protobuf:"varint,4,opt,name=periodSeconds"`
	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// Defaults to 1. Must be 1 for liveness and startup. Minimum value is 1.
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty" protobuf:"varint,5,opt,name=successThreshold"`
	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty" protobuf:"varint,6,opt,name=failureThreshold"`
}

type Handler struct {
	// One and only one of the following should be specified.
	// Exec specifies the action to take.
	// +optional
	Exec *corev1.ExecAction `json:"exec,omitempty" protobuf:"bytes,1,opt,name=exec"`
	// HTTPGet specifies the http request to perform.
	// +optional
	HTTPGet *HTTPGetAction `json:"httpGet,omitempty" protobuf:"bytes,2,opt,name=httpGet"`
}

// HTTPGetAction describes an action based on HTTP Get requests.
type HTTPGetAction struct {
	// Path to access on the HTTP server.
	// +optional
	Path string `json:"path,omitempty" protobuf:"bytes,1,opt,name=path"`
	// Name or number of the port to access on the container.
	// Number must be in the range 1 to 65535.
	// Name must be an IANA_SVC_NAME.
	Port int `json:"port" protobuf:"bytes,2,opt,name=port"`
	// Host name to connect to, defaults to the pod IP. You probably want to set
	// "Host" in httpHeaders instead.
	// +optional
	Host string `json:"host,omitempty" protobuf:"bytes,3,opt,name=host"`
	// Scheme to use for connecting to the host.
	// Defaults to HTTP.
	// +optional
	Scheme corev1.URIScheme `json:"scheme,omitempty" protobuf:"bytes,4,opt,name=scheme,casttype=URIScheme"`
	// Custom headers to set in the request. HTTP allows repeated headers.
	// +optional
	HTTPHeaders []corev1.HTTPHeader `json:"httpHeaders,omitempty" protobuf:"bytes,5,rep,name=httpHeaders"`
}
