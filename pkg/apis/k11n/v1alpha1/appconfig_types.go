package v1alpha1

import (
	"regexp"
	"strings"

	"github.com/spf13/cast"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k11n/konstellation/pkg/utils/files"
)

var (
	allowedEnvVar = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

const (
	ConfigFileName  = "config.yaml"
	ConfigHashLabel = "k11n.dev/configHash"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AppConfig is the Schema for the appconfigs API
// +kubebuilder:resource:path=appconfigs,scope=Cluster
type AppConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ConfigYaml []byte `json:"config"`
}

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

func (c *AppConfig) ToConfigMap() *corev1.ConfigMap {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.ConfigHash(),
			Labels: map[string]string{
				ConfigHashLabel: c.ConfigHash(),
			},
		},
		Data: make(map[string]string),
	}
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
		if allowedEnvVar.MatchString(key) {
			cm.Data[key] = strVal
		}
	}

	// include config.yaml as a file
	cm.Data[ConfigFileName] = string(c.ConfigYaml)

	return &cm
}

func (c *AppConfig) ConfigHash() string {
	return files.Sha1ChecksumString(string(c.ConfigYaml))
}

func NewAppConfig(app, target string) *AppConfig {
	name := app
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
	}
}
