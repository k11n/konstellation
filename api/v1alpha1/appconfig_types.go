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
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	allowedEnvVar = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

const (
	ConfigEnvVar      = "APP_CONFIG"
	ConfigHashLabel   = "k11n.dev/configHash"
	SharedConfigLabel = "k11n.dev/sharedConfig"

	ConfigTypeApp    ConfigType = "app"
	ConfigTypeShared ConfigType = "shared"
)

type ConfigType string

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.type`

// AppConfig is the Schema for the appconfigs API
type AppConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Type       ConfigType `json:"type"`
	ConfigYaml []byte     `json:"config"`
}

// +kubebuilder:object:root=true

// AppConfigList contains a list of AppConfig
type AppConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppConfig `json:"items"`
}

func (c *AppConfig) GetSharedName() string {
	return c.Labels[SharedConfigLabel]
}

func (c *AppConfig) GetAppName() string {
	return c.Labels[AppLabel]
}

func (c *AppConfig) GetTarget() string {
	return c.Labels[TargetLabel]
}

func (c *AppConfig) GetConfig() map[string]interface{} {
	if c.ConfigYaml == nil {
		return nil
	}
	config := make(map[string]interface{})
	yaml.Unmarshal(c.ConfigYaml, config)
	return config
}

func (c *AppConfig) SetConfig(config map[string]interface{}) error {
	content, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	c.ConfigYaml = content
	return nil
}

func (c *AppConfig) SetConfigYAML(conf []byte) error {
	out := yaml.Node{}
	// validate config to be yaml
	if err := yaml.Unmarshal(conf, &out); err != nil {
		return errors.Wrap(err, "config contains invalid YAML")
	}
	c.ConfigYaml = conf
	return nil
}

func (c *AppConfig) MergeWith(other *AppConfig) {
	config := c.GetConfig()
	otherConfig := other.GetConfig()
	if config == nil {
		config = make(map[string]interface{})
	}
	for key, val := range otherConfig {
		config[key] = val
	}
	c.SetConfig(config)
}

func (c *AppConfig) ToEnvMap() map[string]string {
	data := make(map[string]string)
	for key, val := range c.GetConfig() {
		var strVal string
		switch val.(type) {
		case string:
			strVal = val.(string)
		case int64, int, uint, uint64, float32, float64:
			strVal = cast.ToString(val)
		default:
			// not eligible to be an env var
			continue
		}

		// ensure key is valid env chars
		key = strings.ToUpper(key)
		key = strings.ReplaceAll(key, "-", "_")
		if allowedEnvVar.MatchString(key) {
			data[key] = strVal
		}
	}

	// include config.yaml as a file
	data[ConfigEnvVar] = string(c.ConfigYaml)

	return data
}

func NewAppConfig(app, target string) *AppConfig {
	name := fmt.Sprintf("app-%s", app)
	if target != "" {
		name += "-" + target
	}
	return &AppConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				AppLabel:    app,
				TargetLabel: target,
			},
		},
		Type: ConfigTypeApp,
	}
}

func NewSharedConfig(name, target string) *AppConfig {
	resName := "shared-" + name
	return &AppConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: resName,
			Labels: map[string]string{
				SharedConfigLabel: name,
				TargetLabel:       target,
			},
		},
		Type: ConfigTypeShared,
	}
}

func init() {
	SchemeBuilder.Register(&AppConfig{}, &AppConfigList{})
}
