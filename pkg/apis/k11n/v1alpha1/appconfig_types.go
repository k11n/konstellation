package v1alpha1

import (
	"fmt"
	"regexp"
	"strings"

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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AppConfig is the Schema for the appconfigs API
// +kubebuilder:resource:path=appconfigs,scope=Cluster
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.type`
type AppConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Type       ConfigType `json:"type"`
	ConfigYaml []byte     `json:"config"`
}

type ConfigType string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AppConfigList contains a list of AppConfig
type AppConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AppConfig{}, &AppConfigList{})
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
