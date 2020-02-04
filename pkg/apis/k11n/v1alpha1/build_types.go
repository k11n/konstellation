package v1alpha1

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BuildSpec defines the desired state of Build
type BuildSpec struct {
	Registry  string           `json:"registry"`
	Image     string           `json:"image"`
	Tag       string           `json:"tag"`
	CreatedAt metav1.Timestamp `json:"createdAt"`
}

// BuildStatus defines the observed state of Build
type BuildStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Build is the Schema for the builds API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=builds,scope=Cluster
type Build struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuildSpec   `json:"spec,omitempty"`
	Status BuildStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BuildList contains a list of Build
type BuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Build `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Build{}, &BuildList{})
}

func (b *Build) ImagePath() string {
	return fmt.Sprintf("%s/%s", b.Spec.Registry, b.Spec.Image)
}

func (b *Build) FullImageWithTag() string {
	fullImage := b.ImagePath()
	if b.Spec.Tag != "" {
		fullImage += ":" + b.Spec.Tag
	}
	return fullImage
}

func (s *BuildSpec) NameFromSpec() string {
	image := strings.ReplaceAll(s.Image, "/", "-")
	name := fmt.Sprintf("%s-%s", s.Registry, image)
	if s.Tag != "" {
		name += "-" + s.Tag
	}
	return name
}

func NewBuild(registry, image, tag string) *Build {
	b := Build{
		Spec: BuildSpec{
			Registry: registry,
			Image:    image,
			Tag:      tag,
		},
	}
	b.ObjectMeta = metav1.ObjectMeta{
		Name: b.Spec.NameFromSpec(),
	}
	return &b
}
