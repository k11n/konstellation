package resources

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/utils/files"
)

type Exporter struct {
	client      client.Client
	targetPath  string
	encoder     runtime.Encoder
	printStatus bool
}

func NewExporter(kclient client.Client, targetPath string) *Exporter {
	// the schemes should be created prior to creating this
	return &Exporter{
		client:     kclient,
		targetPath: targetPath,
		encoder: json.NewSerializerWithOptions(json.DefaultMetaFactory, nil, nil,
			json.SerializerOptions{
				Yaml:   true,
				Pretty: true,
				Strict: false,
			}),
		printStatus: true,
	}
}

func (e *Exporter) Export() error {
	// create directory structure
	// target/
	//   apps/
	//     app-name.yaml
	//   builds/
	//     build-name.yaml
	//   configs/
	//     app/
	//       app-name.yaml
	//       target/
	//         app-name.yaml
	//     shared/
	//       name.yaml
	//       target/
	//         app-name.yaml

	if err := os.MkdirAll(e.targetPath, files.DefaultDirectoryMode); err != nil {
		return err
	}

	if err := e.ExportApps(path.Join(e.targetPath, "apps")); err != nil {
		return err
	}

	if err := e.ExportBuilds(path.Join(e.targetPath, "builds")); err != nil {
		return err
	}

	if err := e.ExportConfigs(path.Join(e.targetPath, "configs")); err != nil {
		return err
	}

	// export cluster config and nodepool
	cc, err := GetClusterConfig(e.client)
	if err != nil {
		return err
	}
	f, err := os.Create(path.Join(e.targetPath, "clusterconfig.yaml"))
	if err != nil {
		return err
	}
	defer f.Close()
	if err := e.encoder.Encode(cc, f); err != nil {
		return err
	}
	if e.printStatus {
		fmt.Println("exported cluster config")
	}

	nodepools, err := GetNodepools(e.client)
	if err != nil {
		return err
	}
	for _, np := range nodepools {
		f, err := os.Create(path.Join(e.targetPath, np.Name+".yaml"))
		if err != nil {
			return err
		}
		defer f.Close()
		if err := e.encoder.Encode(np, f); err != nil {
			return err
		}
		if e.printStatus {
			fmt.Println("exported nodepool", np.Name)
		}
	}

	return nil
}

func (e *Exporter) ExportApps(appsDir string) error {
	err := os.MkdirAll(appsDir, files.DefaultDirectoryMode)
	if err != nil {
		return err
	}
	err = ForEach(e.client, &v1alpha1.AppList{}, func(item interface{}) error {
		app := item.(v1alpha1.App)
		f, err := os.Create(path.Join(appsDir, app.Name+".yaml"))
		if err != nil {
			return err
		}
		defer f.Close()

		err = e.encoder.Encode(&app, f)
		if err == nil && e.printStatus {
			fmt.Println("exported app", app.Name)
		}
		return err
	})
	return err
}

func (e *Exporter) ExportBuilds(buildsDir string) error {
	err := os.MkdirAll(buildsDir, files.DefaultDirectoryMode)
	if err != nil {
		return err
	}
	err = ForEach(e.client, &v1alpha1.BuildList{}, func(item interface{}) error {
		build := item.(v1alpha1.Build)
		f, err := os.Create(path.Join(buildsDir, build.Name+".yaml"))
		if err != nil {
			return err
		}
		defer f.Close()

		err = e.encoder.Encode(&build, f)
		if err == nil && e.printStatus {
			fmt.Println("exported build", build.Name)
		}
		return err
	})
	return err
}

func (e *Exporter) ExportConfigs(configsDir string) error {
	err := os.MkdirAll(configsDir, files.DefaultDirectoryMode)
	if err != nil {
		return err
	}
	err = ForEach(e.client, &v1alpha1.AppConfigList{}, func(item interface{}) error {
		config := item.(v1alpha1.AppConfig)
		// determine the directory it should be in
		configDir := path.Join(configsDir, string(config.Type))
		if config.GetTarget() != "" {
			configDir = path.Join(configDir, config.GetTarget())
		}

		if err := os.MkdirAll(configDir, files.DefaultDirectoryMode); err != nil {
			return err
		}

		//  append name
		var name string
		if config.Type == v1alpha1.ConfigTypeApp {
			name = config.GetAppName()
		} else {
			name = config.GetSharedName()
		}

		filename := fmt.Sprintf("%s.yaml", path.Join(configDir, name))
		err = ioutil.WriteFile(filename, config.ConfigYaml, files.DefaultFileMode)
		if err == nil && e.printStatus {
			fmt.Printf("exported %s config: %s\n", config.Type, name)
		}
		return err
	})

	return err
}
