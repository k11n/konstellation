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
	"fmt"
	"time"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/k11n/konstellation/pkg/utils/objects"
)

const (
	AppLabel    = "k11n.dev/app"
	TargetLabel = "k11n.dev/target"
)

// AppSpec defines the desired state of App
type AppSpec struct {
	Registry string `json:"registry,omitempty"`

	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// +optional
	ImageTag string `json:"imageTag,omitempty"`

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
	Prometheus *PrometheusSpec `json:"prometheus,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable
	Targets []TargetConfig `json:"targets"`
}

type AppCommonSpec struct {
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
	Dependencies []AppReference `json:"dependencies,omitempty"`

	// +kubebuilder:validation:Optional
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`

	// +kubebuilder:validation:Optional
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// +kubebuilder:validation:Optional
	// +optional
	Probes ProbeConfig `json:"probes,omitempty"`
}

// AppStatus defines the observed state of App
type AppStatus struct {
	ActiveTargets []string `json:"activeTargets,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// App is the Schema for the apps API
type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppSpec   `json:"spec,omitempty"`
	Status AppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AppList contains a list of App
type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App `json:"items"`
}

type PortSpec struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
	// TCP by default. Ingress works only with HTTP services
	// +optional
	Protocol corev1.Protocol `json:"protocol,omitempty"`
}

type ScaleSpec struct {
	TargetCPUUtilization int32 `json:"targetCPUUtilizationPercentage,omitempty"`
	Min                  int32 `json:"min,omitempty"`
	Max                  int32 `json:"max,omitempty"`
	// +optional
	ScaleUp *ScaleBehavior `json:"scaleUp,omitempty"`
	// +optional
	ScaleDown *ScaleBehavior `json:"scaleDown,omitempty"`
}

type ScaleBehavior struct {
	Step  int32 `json:"step,omitempty"`
	Delay int32 `json:"delay,omitempty"`
}

type ProbeConfig struct {
	// +optional
	Liveness *Probe `json:"liveness,omitempty"`
	// +optional
	Readiness *Probe `json:"readiness,omitempty"`
	// +optional
	Startup *Probe `json:"startup,omitempty"`
}

// +kubebuilder:validation:Enum=latest;halt
type DeployMode string

const (
	DeployLatest DeployMode = "latest"
	DeployHalt   DeployMode = "halt"
)

type TargetConfig struct {
	Name string `json:"name"`

	// +optional
	// +kubebuilder:validation:Optional
	DeployMode DeployMode `json:"deployMode,omitempty"`
	// if ingress is needed
	// +optional
	Ingress *IngressConfig `json:"ingress,omitempty"`
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// +optional
	Scale ScaleSpec `json:"scale,omitempty"`
	// +optional
	Probes ProbeConfig `json:"probes,omitempty"`
}

type IngressConfig struct {
	Hosts []string `json:"hosts"`
	// +kubebuilder:validation:Optional
	Paths []string `json:"paths,omitempty"`
	// +kubebuilder:validation:Optional
	Port string `json:"port,omitempty"`

	// when enabled, redirect http traffic to https
	// +optional
	RequireHTTPS bool `json:"requireHttps,omitempty"`

	// custom annotations for the Ingress
	// +optional
	// +kubebuilder:validation:Optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type AppReference struct {
	Name string `json:"name"`

	// +optional
	Target string `json:"target,omitempty"`
	// +optional
	Port string `json:"port,omitempty"`
}

type PrometheusSpec struct {
	// +kubebuilder:validation:Required
	Endpoints []promv1.Endpoint `json:"endpoints"`

	// +kubebuilder:validation:Optional
	Rules []promv1.Rule `json:"rules,omitempty"`
}

func (a *AppSpec) ScaleSpecForTarget(target string) *ScaleSpec {
	scale := a.Scale.DeepCopy()
	tc := a.GetTargetConfig(target)

	if tc != nil {
		objects.MergeObject(scale, &tc.Scale)
	}
	if scale.Min == 0 {
		scale.Min = 1
	}
	if scale.Max == 0 {
		scale.Max = scale.Min
	}
	return scale
}

func (a *AppSpec) ResourcesForTarget(target string) *corev1.ResourceRequirements {
	res := a.Resources.DeepCopy()
	tc := a.GetTargetConfig(target)
	if tc != nil {
		objects.MergeObject(res, &tc.Resources)
	}
	return res
}

func (a *AppSpec) ProbesForTarget(target string) *ProbeConfig {
	probes := a.Probes.DeepCopy()
	tc := a.GetTargetConfig(target)
	if tc != nil {
		objects.MergeObject(probes, &tc.Probes)
	}
	return probes
}

func (a *AppSpec) DeployModeForTarget(target string) DeployMode {
	deployMode := DeployLatest
	tc := a.GetTargetConfig(target)
	if tc != nil && tc.DeployMode != "" {
		deployMode = tc.DeployMode
	}
	return deployMode
}

func (a *AppSpec) GetTargetConfig(target string) *TargetConfig {
	var targetConf *TargetConfig
	for i := range a.Targets {
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

func (p *Probe) ToCoreProbe() *corev1.Probe {
	coreHander := corev1.Handler{
		Exec: p.Handler.Exec,
	}
	if p.Handler.HTTPGet != nil {
		hg := p.Handler.HTTPGet
		coreHander.HTTPGet = &corev1.HTTPGetAction{
			Path:        hg.Path,
			Host:        hg.Host,
			Port:        intstr.FromString(hg.Port),
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

func (p *ProbeConfig) GetReadinessTimeout() time.Duration {
	timeout := int32(60)
	if p.Readiness != nil {
		if p.Readiness.InitialDelaySeconds > 0 {
			timeout = p.Readiness.InitialDelaySeconds
		} else if p.Readiness.PeriodSeconds > 0 {
			timeout = p.Readiness.PeriodSeconds + p.Readiness.TimeoutSeconds
		}
	}
	return time.Second * time.Duration(timeout)
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
	// Name of the port to access on the container.
	// Name must be an IANA_SVC_NAME.
	Port string `json:"port"`
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

func init() {
	SchemeBuilder.Register(&App{}, &AppList{})
}
