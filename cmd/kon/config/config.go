package config

import (
	"fmt"
	"os"
	"path"

	"gopkg.in/yaml.v2"

	"github.com/davidzhao/konstellation/pkg/components"
	"github.com/davidzhao/konstellation/pkg/components/istio"
	"github.com/davidzhao/konstellation/pkg/components/kubedash"
)

var (
	defaultConfigDir = os.ExpandEnv("$HOME/.konstellation")
	config           *ClientConfig
	Components       = []components.ComponentInstaller{
		&istio.IstioInstaller{},
		&kubedash.KubeDash{},
	}
)

const (
	configName     = "config.yaml"
	ExecutableName = "kon"
)

type ClusterLocation struct {
	Cloud  string
	Region string
}

type ClientConfig struct {
	Clouds struct {
		AWS AWSConfig `yaml:"aws,omitempty"`
	} `yaml:"clouds,omitempty"`
	Clusters        map[string]*ClusterLocation
	SelectedCluster string

	persisted bool
}

func GetConfig() *ClientConfig {
	if config == nil {
		config = &ClientConfig{}
		if err := config.loadFromDisk(); err != nil {
			// no existing config found, set defaults
		} else {
			config.persisted = true
		}
	}
	return config
}

func (c *ClientConfig) loadFromDisk() error {
	file, err := os.Open(c.ConfigFile())
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	return decoder.Decode(c)
}

func (c *ClientConfig) IsPersisted() bool {
	return c.persisted
}

func (c *ClientConfig) IsSetup() bool {
	// return if any of the cloud providers are setup
	if c.Clouds.AWS.IsSetup() {
		return true
	}
	return false
}

func (c *ClientConfig) IsClusterSelected() bool {
	return c.SelectedCluster != ""
}

func (c *ClientConfig) GetClusterLocation(cluster string) (*ClusterLocation, error) {
	cl := c.Clusters[cluster]
	if cl == nil {
		return nil, fmt.Errorf("Could not find cluster %s", cluster)
	}
	return cl, nil
}

func (c *ClientConfig) ToYAML() (str string, err error) {
	content, err := yaml.Marshal(c)
	if err != nil {
		return
	}
	str = string(content)
	return
}

func (c *ClientConfig) ConfigFile() string {
	return path.Join(defaultConfigDir, configName)
}

func (c *ClientConfig) Persist() error {
	if _, err := os.Stat(defaultConfigDir); err != nil {
		// create directory
		err = os.MkdirAll(defaultConfigDir, 0700)
		if err != nil {
			return err
		}
	}
	file, err := os.Create(c.ConfigFile())
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := yaml.NewEncoder(file)
	err = encoder.Encode(c)
	if err == nil {
		c.persisted = true
	}
	return err
}
