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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ComponentSpec struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ComponentConfig map[string]string

// ClusterConfigSpec defines the desired state of ClusterConfig
type ClusterConfigSpec struct {
	Version     string `json:"version"`
	KubeVersion string `json:"kubeVersion"`
	Cloud       string `json:"cloud"`
	Region      string `json:"region"`
	EnableIpv6  bool   `json:"enableIpv6"`
	// +kubebuilder:validation:Optional
	// +nullable
	AWS *AWSClusterSpec `json:"aws"`
	// +kubebuilder:validation:Optional
	// +nullable
	Targets []string `json:"targets"`
	// +kubebuilder:validation:Optional
	// +nullable
	ComponentConfig map[string]ComponentConfig `json:"componentConfig"`
}

// ClusterConfigStatus defines the observed state of ClusterConfig
type ClusterConfigStatus struct {
	InstalledComponents []ComponentSpec `json:"components"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=clusterconfigs,scope=Cluster

// ClusterConfig is the Schema for the clusterconfigs API
type ClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterConfigSpec   `json:"spec,omitempty"`
	Status ClusterConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterConfigList contains a list of ClusterConfig
type ClusterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterConfig `json:"items"`
}

type AWSClusterSpec struct {
	// input values
	VpcCidr           string      `json:"vpcCidr"`
	AvailabilityZones []string    `json:"availabilityZone"`
	Topology          AWSTopology `json:"topology"`
	// +kubebuilder:validation:Optional
	// +nullable
	AdminGroups []string `json:"adminGroups,omitempty"`
	// optional, only if it's using an existing VPC
	Vpc string `json:"vpc"`

	// set after cluster is created
	Ipv6Cidr       string       `json:"ipv6Cidr,omitempty"`
	SecurityGroups []string     `json:"securityGroups"`
	PublicSubnets  []*AWSSubnet `json:"publicSubnets"`
	// +kubebuilder:validation:Optional
	// +nullable
	PrivateSubnets []*AWSSubnet `json:"privateSubnets"`
	AlbRoleArn     string       `json:"albRoleArn"`
	NodeRoleArn    string       `json:"nodeRoleArn"`
	AdminRoleArn   string       `json:"adminRoleArn"`
}

type AWSSubnet struct {
	SubnetId         string `json:"subnetId"`
	IsPublic         bool   `json:"isPublic"`
	Ipv4Cidr         string `json:"ipv4Cidr"`
	Ipv6Cidr         string `json:"ipv6Cidr,omitempty"`
	AvailabilityZone string `json:"availabilityZone"`
}

type AWSTopology string

const (
	AWSTopologyPublic        AWSTopology = "public"
	AWSTopologyPublicPrivate AWSTopology = "public_private"
)

func (c *ClusterConfig) GetComponentConfig(name string) ComponentConfig {
	// check status, if not installed, return nil
	installed := false
	for _, comp := range c.Status.InstalledComponents {
		if comp.Name == name {
			installed = true
			break
		}
	}

	if !installed {
		return nil
	}

	conf := c.Spec.ComponentConfig[name]
	if conf == nil {
		conf = map[string]string{}
	}
	return conf
}

func (cs *AWSClusterSpec) NumZones() int {
	return len(cs.AvailabilityZones)
}

func init() {
	SchemeBuilder.Register(&ClusterConfig{}, &ClusterConfigList{})
}
