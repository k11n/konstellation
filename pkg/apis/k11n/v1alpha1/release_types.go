package v1alpha1

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ReleaseSpec defines the desired state of Build
type ReleaseSpec struct {
	Registry  string           `json:"registry"`
	Image     string           `json:"image"`
	Tag       string           `json:"tag"`
	CreatedAt metav1.Timestamp `json:"createdAt"`
}

// ReleaseStatus defines the observed state of Build
type ReleaseStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Release is the Schema for the releases API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=releases,scope=Cluster
type Release struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReleaseSpec   `json:"spec,omitempty"`
	Status ReleaseStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReleaseList contains a list of Release
type ReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Release `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Release{}, &ReleaseList{})
}

func (b *Release) ImagePath() string {
	return fmt.Sprintf("%s/%s", b.Spec.Registry, b.Spec.Image)
}

func (b *Release) FullImageWithTag() string {
	fullImage := b.ImagePath()
	if b.Spec.Tag != "" {
		fullImage += ":" + b.Spec.Tag
	}
	return fullImage
}

func (b *Release) ShortName() string {
	name := b.Spec.Image
	if b.Spec.Tag != "" {
		name += ":" + b.Spec.Tag
	}
	return name
}

func (s *ReleaseSpec) NameFromSpec() string {
	image := strings.ReplaceAll(s.Image, "/", "-")
	name := fmt.Sprintf("%s-%s", s.Registry, image)
	if s.Tag != "" {
		name += "-" + s.Tag
	}
	return name
}

func NewRelease(registry, image, tag string) *Release {
	b := Release{
		Spec: ReleaseSpec{
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
