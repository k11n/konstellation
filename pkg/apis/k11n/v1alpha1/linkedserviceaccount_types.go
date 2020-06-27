package v1alpha1

import (
	"fmt"

	"github.com/mitchellh/hashstructure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SpecHashAnnotation = "k11n.dev/lsaSpecHash"
)

// LinkedServiceAccountSpec defines the desired state of LinkedServiceAccount
type LinkedServiceAccountSpec struct {
	Targets []string `json:"targets"`
	// +kubebuilder:validation:Optional
	AWS *LinkedServiceAccountAWSSpec `json:"aws,omitempty"`
}

type LinkedServiceAccountAWSSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems:=1
	PolicyARNs []string `json:"policyArns"`
}

// ConnectedServiceAccountStatus defines the observed state of LinkedServiceAccount
type LinkedServiceAccountStatus struct {
	LinkedTargets []string `json:"linkedTargets,omitempty"` // list of targets that are linked
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinkedServiceAccount is the Schema for the linkedserviceaccounts API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=linkedserviceaccounts,scope=Cluster
type LinkedServiceAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinkedServiceAccountSpec   `json:"spec,omitempty"`
	Status LinkedServiceAccountStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinkedServiceAccountList contains a list of ConnectedServiceAccount
type LinkedServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinkedServiceAccount `json:"items"`
}

func (l *LinkedServiceAccount) NeedsReconcile() (bool, error) {
	hashVal, err := hashstructure.Hash(&l.Spec, nil)
	if err != nil {
		return false, err
	}

	if l.Annotations == nil {
		return true, nil
	}
	return l.Annotations[SpecHashAnnotation] != fmt.Sprintf("%d", hashVal), nil
}

func (l *LinkedServiceAccount) UpdateHash() error {
	hashVal, err := hashstructure.Hash(&l.Spec, nil)
	if err != nil {
		return err
	}

	if l.Annotations == nil {
		l.Annotations = map[string]string{}
	}
	l.Annotations[SpecHashAnnotation] = fmt.Sprintf("%d", hashVal)
	return nil
}

func (l *LinkedServiceAccount) GetPolicies() []string {
	if l.Spec.AWS != nil {
		return l.Spec.AWS.PolicyARNs
	}
	return []string{}
}

func init() {
	SchemeBuilder.Register(&LinkedServiceAccount{}, &LinkedServiceAccountList{})
}
