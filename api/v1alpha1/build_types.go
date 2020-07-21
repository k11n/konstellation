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
	"log"
	"regexp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k11n/konstellation/pkg/utils/files"
)

var (
	allowedNameRegexp = regexp.MustCompile(`[^a-zA-Z0-9\-]`)
)

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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=builds,scope=Cluster
// +kubebuilder:printcolumn:name="Registry",type=string,JSONPath=`.spec.registry`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
// +kubebuilder:printcolumn:name="Tag",type=string,JSONPath=`.spec.tag`

// Build is the Schema for the builds API
type Build struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuildSpec   `json:"spec,omitempty"`
	Status BuildStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BuildList contains a list of Build
type BuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Build `json:"items"`
}

// sha1 hash of full name
func (b *Build) GetUniqueName() string {
	sb := bytes.NewBuffer(nil)
	sb.WriteString(b.Spec.Registry)
	sb.WriteString(":")
	sb.WriteString(b.FullImageWithTag())
	checksum, err := files.Sha1Checksum(sb)
	if err != nil {
		log.Fatalf("Unable to generate checksum: %v", err)
	} else {
		checksum = checksum[:4]
	}
	name := b.Spec.Image
	if b.Spec.Tag != "" {
		name += "-" + b.Spec.Tag
	}
	name = allowedNameRegexp.ReplaceAllString(name, "-") + "-" + checksum

	return name
}

func (b *Build) ImagePath() string {
	if b.Spec.Registry != "" {
		return fmt.Sprintf("%s/%s", b.Spec.Registry, b.Spec.Image)
	}
	return b.Spec.Image
}

func (b *Build) FullImageWithTag() string {
	fullImage := b.ImagePath()
	if b.Spec.Tag != "" {
		fullImage += ":" + b.Spec.Tag
	}
	return fullImage
}

func (b *Build) ShortName() string {
	name := b.Spec.Image
	if b.Spec.Tag != "" {
		name += ":" + b.Spec.Tag
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
		Name: b.GetUniqueName(),
	}
	return &b
}

func init() {
	SchemeBuilder.Register(&Build{}, &BuildList{})
}
