package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AppReleaseSpec defines a release of AppTarget
type AppReleaseSpec struct {
	App    string `json:"app"`
	Target string `json:"target"`
	Build  string `json:"build"`
	Config string `json:"config"`

	// num desired default state, autoscaling could change desired in status
	NumDesired        int32       `json:"numDesired"`
	Role              ReleaseRole `json:"role"`
	TrafficPercentage int32       `json:"trafficPercentage"`

	AppCommonSpec `json:",inline"`
}

// AppReleaseStatus defines the observed state of AppRelease
type AppReleaseStatus struct {
	State          ReleaseState `json:"state"`
	StateChangedAt metav1.Time  `json:"stateChangedAt"`
	NumDesired     int32        `json:"numDesired"`
	NumReady       int32        `json:"numReady"`
	NumAvailable   int32        `json:"numAvailable"`
	Reason         string       `json:"reason"`
}

type ReleaseState string

func (rs ReleaseState) String() string {
	return string(rs)
}

const (
	ReleaseStateNew       ReleaseState = "new"
	ReleaseStateCanarying ReleaseState = "canarying"
	ReleaseStateReleasing ReleaseState = "releasing"
	ReleaseStateReleased  ReleaseState = "released"
	ReleaseStateRetiring  ReleaseState = "retiring"
	ReleaseStateRetired   ReleaseState = "retired"
	ReleaseStateFailed    ReleaseState = "failed"
	ReleaseStateBad       ReleaseState = "bad"
)

type ReleaseRole string

func (rr ReleaseRole) String() string {
	return string(rr)
}

const (
	// no special role
	ReleaseRoleNone = ""
	// indicates the release that should be serving all of current traffic
	ReleaseRoleActive = "active"
	// indicates the release we are moving towards
	ReleaseRoleTarget = "target"
	ReleaseRoleBad    = "bad"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AppRelease is the Schema for the appreleases API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=appreleases,scope=Namespaced
type AppRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppReleaseSpec   `json:"spec,omitempty"`
	Status AppReleaseStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AppReleaseList contains a list of AppRelease
type AppReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppRelease `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AppRelease{}, &AppReleaseList{})
}

func (s *AppReleaseSpec) ContainerPorts() []corev1.ContainerPort {
	ports := []corev1.ContainerPort{}
	for _, p := range s.Ports {
		ports = append(ports, corev1.ContainerPort{
			Name:          p.Name,
			ContainerPort: p.Port,
			Protocol:      p.Protocol,
		})
	}
	return ports
}
