package commands

import (
	"context"
	"fmt"
	"log"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/cmd/kon/config"
	"github.com/k11n/konstellation/cmd/kon/kube"
	"github.com/k11n/konstellation/cmd/kon/providers"
	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/resources"
	utilscli "github.com/k11n/konstellation/pkg/utils/cli"
)

func getActiveCluster() (*activeCluster, error) {
	if err := ensureClusterSelected(); err != nil {
		return nil, err
	}

	conf := config.GetConfig()
	cm, err := ClusterManagerForCluster(conf.SelectedCluster)
	if err != nil {
		return nil, err
	}

	ac := activeCluster{
		Manager: cm,
		Cluster: conf.SelectedCluster,
	}

	err = ac.initClient()
	if err != nil {
		return nil, err
	}
	return &ac, nil
}

type activeCluster struct {
	Manager providers.ClusterManager
	Cluster string
	kclient client.Client
}

func (c *activeCluster) loadResourcesIntoKube() error {
	// load new resources into the current cluster
	fmt.Println("Loading custom resource definitions into Kubernetes...")
	contextName := resources.ContextNameForCluster(c.Manager.Cloud(), c.Cluster)
	for _, file := range kube.KUBE_RESOURCES {
		err := utilscli.KubeApplyFromBox(file, contextName)
		if err != nil {
			return errors.Wrapf(err, "Unable to apply config %s", file)
		}
	}

	err := utils.WaitUntilComplete(utils.ShortTimeoutSec, utils.MediumCheckInterval, func() (bool, error) {
		// use a new kclient to avoid caching
		kclient, err := kube.KubernetesClientWithContext(contextName)
		if err != nil {
			return false, err
		}
		_, err = resources.GetClusterConfig(kclient)
		if err == nil || err == resources.ErrNotFound {
			// object already there or type created
			return true, nil
		}

		// likely it hasn't loaded the type yet
		log.Printf("Custom resources still not created. %v", err)
		return false, nil
	})
	if err != nil {
		return errors.Wrap(err, "Failed to load required resources into Kube")
	}
	// need to reset after resources are just loaded to avoid caching
	c.kclient = nil

	return nil
}

// check nodepool status, if ready continue
func (c *activeCluster) configureNodepool() error {
	kclient := c.kubernetesClient()

	// cluster config must be present now, load it and pass it into nodegroups
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}

	cloud := GetCloud(cc.Spec.Cloud)
	generator, err := PromptClusterGenerator(cloud, cc.Spec.Region)
	if err != nil {
		return err
	}

	// prompt for nodepool config
	np, err := generator.CreateNodepoolConfig(cc)
	if err != nil {
		return err
	}

	cm := NewClusterManager(cc.Spec.Cloud, cc.Spec.Region)

	err = cm.CreateNodepool(cc, np)
	if err != nil {
		return err
	}

	// save spec to Kube
	if _, err = resources.UpdateResource(kclient, np, nil, nil); err != nil {
		return err
	}

	fmt.Printf("Successfully created nodepool %s\n", np.GetObjectMeta().GetName())
	return nil
}

func (c *activeCluster) installComponents(force bool) error {
	kclient := c.kubernetesClient()
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}

	messagesPrinted := false
	// now install all these resources
	installed := make(map[string]string)
	for _, comp := range cc.Status.InstalledComponents {
		installed[comp.Name] = comp.Version
	}

	provider := GetCloud(c.Manager.Cloud())
	for _, compInstaller := range provider.GetComponents() {
		if !force && installed[compInstaller.Name()] != "" {
			continue
		}
		if !messagesPrinted {
			messagesPrinted = true
			fmt.Println("\nInstalling required components onto the current cluster...")
		}
		fmt.Println("\nInstalling Kubernetes components for", compInstaller.Name())

		compVersion := compInstaller.VersionForKube(cc.Spec.KubeVersion)
		err = compInstaller.InstallComponent(kclient)
		if err != nil {
			return err
		}

		installed[compInstaller.Name()] = compVersion
	}

	// mark it as installed
	cc.Status.InstalledComponents = make([]v1alpha1.ComponentSpec, 0, len(installed))
	for key, val := range installed {
		cc.Status.InstalledComponents = append(cc.Status.InstalledComponents, v1alpha1.ComponentSpec{
			Name:    key,
			Version: val,
		})
	}

	err = kclient.Status().Update(context.Background(), cc)
	if err != nil {
		return err
	}
	return nil
}

func (c *activeCluster) kubernetesClient() client.Client {
	if c.kclient == nil {
		err := c.initClient()
		if err != nil {
			log.Fatalf("Unable to acquire client to Kubernetes, err: %v", err)
		}
	}
	return c.kclient
}

func (c *activeCluster) initClient() error {
	kclient, err := kube.KubernetesClientWithContext(resources.ContextNameForCluster(c.Manager.Cloud(), c.Cluster))
	if err != nil {
		return errors.Wrap(err, "Unable to create Kubernetes Client")
	}
	c.kclient = kclient
	return nil
}
