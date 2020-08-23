package resources

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/utils/files"
)

// handles exporting and importing cluster settings to files
// the exported data should be in this structure
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
//   linkedaccounts/
//     account-name.yaml

type Exporter struct {
	client      client.Client
	targetPath  string
	encoder     runtime.Encoder
	printStatus bool
}

type Importer struct {
	client      client.Client
	sourcePath  string
	decoder     runtime.Decoder
	printStatus bool
}

func NewExporter(kclient client.Client, targetPath string) *Exporter {
	// the schemes should be created prior to creating this
	return &Exporter{
		client:      kclient,
		targetPath:  targetPath,
		encoder:     NewYAMLEncoder(),
		printStatus: true,
	}
}

func NewImporter(kclient client.Client, sourcePath string) *Importer {
	return &Importer{
		client:      kclient,
		sourcePath:  sourcePath,
		decoder:     NewYAMLDecoder(),
		printStatus: true,
	}
}

func (e *Exporter) Export() error {
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

	if err := e.ExportLinkedAccounts(path.Join(e.targetPath, "linkedaccounts")); err != nil {
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
	e.cleanupMeta(&cc.ObjectMeta)
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
		e.cleanupMeta(&np.ObjectMeta)
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

		e.cleanupMeta(&app.ObjectMeta)
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

		e.cleanupMeta(&build.ObjectMeta)
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

func (e *Exporter) ExportLinkedAccounts(dir string) error {
	err := os.MkdirAll(dir, files.DefaultDirectoryMode)
	if err != nil {
		return err
	}

	err = ForEach(e.client, &v1alpha1.LinkedServiceAccountList{}, func(item interface{}) error {
		lsa := item.(v1alpha1.LinkedServiceAccount)
		f, err := os.Create(path.Join(dir, lsa.Name+".yaml"))
		if err != nil {
			return err
		}
		defer f.Close()

		e.cleanupMeta(&lsa.ObjectMeta)
		// don't store annotations with these
		lsa.Annotations = nil
		err = e.encoder.Encode(&lsa, f)
		if err == nil && e.printStatus {
			fmt.Println("exported linked service account", lsa.Name)
		}
		return err
	})
	return err
}

func (e *Exporter) cleanupMeta(meta *metav1.ObjectMeta) {
	meta.ResourceVersion = ""
	meta.Generation = 0
	meta.UID = ""
	meta.CreationTimestamp = metav1.Time{}
	meta.SetSelfLink("")
}

func (i *Importer) ImportApps() error {
	appsDir := path.Join(i.sourcePath, "apps")
	files, err := ioutil.ReadDir(appsDir)
	if err != nil {
		// if dir isn't there, ignore
		return nil
	}

	for _, f := range files {
		if strings.HasPrefix(f.Name(), ".") {
			continue
		} else if f.IsDir() {
			fmt.Println("Unexpected directory", f.Name())
			continue
		}
		obj, err := ReadObjectFromFile(i.decoder, path.Join(appsDir, f.Name()), &v1alpha1.App{})
		if err != nil {
			return err
		}

		app := obj.(*v1alpha1.App)

		// load into cluster
		if _, err = UpdateResource(i.client, app, nil, nil); err != nil {
			return errors.Wrapf(err, "could not import app: %s", app.Name)
		}
		if i.printStatus {
			fmt.Println("Imported app", app.Name)
		}
	}
	return nil
}

func (i *Importer) ImportBuilds() error {
	buildsDir := path.Join(i.sourcePath, "builds")
	files, err := ioutil.ReadDir(buildsDir)
	if err != nil {
		// if dir isn't there, ignore
		return nil
	}

	for _, f := range files {
		if strings.HasPrefix(f.Name(), ".") {
			continue
		} else if f.IsDir() {
			fmt.Println("Unexpected directory", f.Name())
			continue
		}
		obj, err := ReadObjectFromFile(i.decoder, path.Join(buildsDir, f.Name()), &v1alpha1.Build{})
		if err != nil {
			return err
		}

		build := obj.(*v1alpha1.Build)

		// load into cluster
		if _, err = UpdateResource(i.client, build, nil, nil); err != nil {
			return err
		}
		if i.printStatus {
			fmt.Println("Imported build", build.Name)
		}
	}
	return nil
}

type configImport struct {
	dir      string
	confType v1alpha1.ConfigType
}

func (i *Importer) ImportConfigs() error {
	configsDir := path.Join(i.sourcePath, "configs")
	configSets := []configImport{
		{
			dir:      path.Join(configsDir, "app"),
			confType: v1alpha1.ConfigTypeApp,
		},
		{
			dir:      path.Join(configsDir, "shared"),
			confType: v1alpha1.ConfigTypeShared,
		},
	}

	for _, ci := range configSets {
		if _, err := os.Stat(ci.dir); err != nil {
			continue
		}

		files, err := ioutil.ReadDir(ci.dir)
		if err != nil {
			return err
		}

		// import files directly, and directories as targets
		for _, f := range files {
			if strings.HasPrefix(f.Name(), ".") {
				continue
			}
			itemPath := path.Join(ci.dir, f.Name())
			if f.IsDir() {
				subfiles, err := ioutil.ReadDir(itemPath)
				if err != nil {
					return err
				}
				target := f.Name()
				for _, subf := range subfiles {
					if strings.HasPrefix(f.Name(), ".") {
						continue
					} else if subf.IsDir() {
						return fmt.Errorf("unexpected directory: %s", path.Join(itemPath, subf.Name()))
					}
					if err = i.importConfig(path.Join(itemPath, subf.Name()), ci.confType, target); err != nil {
						return err
					}
					if i.printStatus {
						fmt.Printf("Imported %s config %s (target %s)\n", ci.confType, subf.Name(), target)
					}
				}
			} else {
				if err = i.importConfig(itemPath, ci.confType, ""); err != nil {
					return err
				}
				if i.printStatus {
					fmt.Printf("Imported %s config %s\n", ci.confType, f.Name())
				}
			}
		}
	}
	return nil
}

func (i *Importer) ImportLinkedAccounts() error {
	dir := path.Join(i.sourcePath, "linkedaccounts")
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		// if dir isn't there, ignore
		return nil
	}

	for _, f := range files {
		if strings.HasPrefix(f.Name(), ".") {
			continue
		} else if f.IsDir() {
			fmt.Println("Unexpected directory", f.Name())
			continue
		}

		obj, err := ReadObjectFromFile(i.decoder, path.Join(dir, f.Name()), &v1alpha1.LinkedServiceAccount{})
		if err != nil {
			return err
		}

		lsa := obj.(*v1alpha1.LinkedServiceAccount)

		// load into cluster
		if _, err = UpdateResource(i.client, lsa, nil, nil); err != nil {
			return err
		}
		if i.printStatus {
			fmt.Println("Imported linked service account", lsa.Name)
		}
	}
	return nil
}

func (i *Importer) importConfig(filename string, confType v1alpha1.ConfigType, target string) error {
	name := path.Base(filename)
	var extension = filepath.Ext(name)
	name = name[0 : len(name)-len(extension)]

	var conf *v1alpha1.AppConfig
	if confType == v1alpha1.ConfigTypeApp {
		conf = v1alpha1.NewAppConfig(name, target)
	} else {
		conf = v1alpha1.NewSharedConfig(name, target)
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	if err = conf.SetConfigYAML(data); err != nil {
		return errors.Wrapf(err, "failed to import config: %s", filename)
	}

	return SaveAppConfig(i.client, conf)
}
