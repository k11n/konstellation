package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	k8sJson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/providers"
	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/cloud/types"
	"github.com/davidzhao/konstellation/pkg/components"
	"github.com/davidzhao/konstellation/pkg/resources"
	"github.com/davidzhao/konstellation/pkg/utils/files"
	"github.com/davidzhao/konstellation/version"
)

var (
	clusterCloudFlag = &cli.StringFlag{
		Name:  "cloud",
		Usage: "the cloud that the cluster resides",
		Value: "aws",
		// Required: true,
	}
	clusterNameFlag = &cli.StringFlag{
		Name:     "cluster",
		Usage:    "cluster name",
		Required: true,
	}
)

var ClusterCommands = []*cli.Command{
	{
		Name:  "cluster",
		Usage: "Kubernetes cluster management",
		Subcommands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "list clusters",
				Action: clusterList,
			},
			{
				Name:  "select",
				Usage: "select an active cluster to work with",
				Action: func(c *cli.Context) error {
					return clusterSelect(c.String("cluster"))
				},
				Flags: []cli.Flag{
					clusterNameFlag,
				},
			},
			{
				Name:   "configure",
				Usage:  "configure cluster settings",
				Action: clusterConfigure,
			},
			{
				Name:   "create",
				Usage:  "creates a cluster",
				Action: clusterCreate,
			},
			{
				Name:   "destroy",
				Usage:  "destroys a cluster",
				Action: clusterDestroy,
				Flags: []cli.Flag{
					clusterNameFlag,
				},
			},
			{
				Name:   "reset",
				Usage:  "resets current active cluster",
				Action: clusterReset,
			},
			{
				Name:   "get-token",
				Usage:  "returns a kubernetes compatible token",
				Action: clusterGetToken,
				Hidden: true,
				Flags: []cli.Flag{
					clusterNameFlag,
				},
			},
		},
	},
}

type clusterInfo struct {
	Cluster *types.Cluster
	Config  *v1alpha1.ClusterConfig
}

func clusterList(c *cli.Context) error {
	conf := config.GetConfig()
	if conf.IsClusterSelected() {
		fmt.Printf("\nSelected cluster %s\n", conf.SelectedCluster)
	}
	for _, cm := range GetClusterManagers() {
		ksvc := cm.KubernetesProvider()
		if ksvc == nil {
			continue
		}
		clusters, err := ksvc.ListClusters(context.Background())
		if err != nil {
			return err
		}

		infos := []*clusterInfo{}
		for _, cluster := range clusters {
			info := &clusterInfo{
				Cluster: cluster,
			}
			infos = append(infos, info)

			contextName := resources.ContextNameForCluster(cm.Cloud(), cluster.Name)
			kclient, err := KubernetesClientWithContext(contextName)
			if err != nil {
				continue
			}
			config, err := resources.GetClusterConfig(kclient)
			if err != nil {
				continue
			}
			info.Config = config
		}
		printClusterSection(cm, infos)
	}

	return nil
}

func clusterCreate(c *cli.Context) error {
	if !config.GetConfig().IsSetup() {
		return fmt.Errorf("Konstellation has not been setup yet. Run `kon setup`")
	}
	fmt.Println(CLUSTER_CREATE_HELP)

	// update existing cluster names to ensure there's no conflict
	if err := updateClusterLocations(); err != nil {
		return err
	}

	clusterConfigFile := path.Join(config.StateDir(), "clusterconfig.yaml")
	nodepoolConfigFile := path.Join(config.StateDir(), "nodepoolconfig.yaml")

	// load persisted state on disk in case it wasn't completed
	cc, nodepool, err := loadExistingConfigs(clusterConfigFile, nodepoolConfigFile)

	var cm providers.ClusterManager
	useExistingConfig := false
	if err == nil {
		// found existing config, ask if user wants to use it
		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Found interrupted cluster creation for %s, resume", cc.Name),
			IsConfirm: true,
			Default:   "y",
		}
		utils.FixPromptBell(&prompt)
		if _, err := prompt.Run(); err == nil {
			useExistingConfig = true
			cm = NewClusterManager(cc.Spec.Cloud, cc.Spec.Region)
		} else if err != promptui.ErrAbort {
			return err
		}
	}

	// configurator
	if !useExistingConfig {
		cm, err = ChooseClusterManagerPrompt("Where would you like to create the cluster?")
		if err != nil {
			return err
		}
		cloud := GetCloud(cm.Cloud())
		generator, err := PromptClusterGenerator(cloud, cm.Region())
		if err != nil {
			return err
		}
		cc, err = generator.CreateClusterConfig()
		if err != nil {
			return err
		}
		err = utils.SaveKubeObject(GetKubeEncoder(), cc, clusterConfigFile)
		if err != nil {
			return err
		}

		nodepool, err = generator.CreateNodepoolConfig(cc)
		if err != nil {
			return err
		}
		err = utils.SaveKubeObject(GetKubeEncoder(), nodepool, nodepoolConfigFile)
		if err != nil {
			return err
		}
	}

	// create cluster and nodepool
	err = cm.CreateCluster(cc)
	if err != nil {
		return err
	}

	err = cm.CreateNodepool(cc, nodepool)
	if err != nil {
		return err
	}

	// load CRD types into cluster and set it up
	ac := activeCluster{
		Manager: cm,
		Cluster: cc.Name,
	}

	// generate config to use with Kube
	updateClusterLocations()
	if err = ac.generateKubeConfig(); err != nil {
		return err
	}

	if err = ac.loadResourcesIntoKube(); err != nil {
		return err
	}
	if err = resources.SaveClusterConfig(ac.kubernetesClient(), cc); err != nil {
		return err
	}
	if err = resources.SaveNodepool(ac.kubernetesClient(), nodepool); err != nil {
		return err
	}

	// delete state files to clear up half completed state
	os.Remove(clusterConfigFile)
	os.Remove(nodepoolConfigFile)

	fmt.Println()
	fmt.Printf("Cluster %s has been successfully created. Run `kon select cluster --cluster %s` to use it\n", cc.Name, cc.Name)
	return nil
}

func clusterDestroy(c *cli.Context) error {
	clusterName := c.String("cluster")
	// update clusters
	if err := updateClusterLocations(); err != nil {
		return err
	}
	cm, err := ClusterManagerForCluster(clusterName)

	fmt.Printf("This will destroy the cluster %s, removing all of the nodepools.\n", clusterName)

	prompt := promptui.Prompt{
		Label: fmt.Sprintf("Sure you want to proceed? (type in %s to proceed)", clusterName),
		Validate: func(v string) error {
			if v != clusterName {
				return fmt.Errorf("Confirmation didn't match %s", clusterName)
			}
			return nil
		},
	}
	utils.FixPromptBell(&prompt)
	_, err = prompt.Run()
	if err != nil {
		return err
	}

	return cm.DeleteCluster(clusterName)
}

func clusterSelect(clusterName string) error {
	if err := updateClusterLocations(); err != nil {
		return err
	}

	cm, err := ClusterManagerForCluster(clusterName)
	if err != nil {
		return err
	}
	ac := activeCluster{
		Manager: cm,
		Cluster: clusterName,
	}

	err = ac.generateKubeConfig()
	if err != nil {
		return err
	}

	conf := config.GetConfig()
	conf.SelectedCluster = clusterName
	err = conf.Persist()
	if err != nil {
		return err
	}

	kclient := ac.kubernetesClient()
	// see if we have to configure cluster
	_, err = resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}
	// TODO: in release versions don't reload resources
	// still load the resources
	err = ac.loadResourcesIntoKube()
	if err != nil {
		return err
	}

	// see if we have a nodepool
	pools, err := resources.GetNodepools(kclient)
	if len(pools) == 0 {
		fmt.Println()
		fmt.Println("Your cluster requires a nodepool to function, let's create that now")
		err := ac.configureNodepool()
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}

	return ac.installComponents()
}

func clusterReset(c *cli.Context) error {
	conf := config.GetConfig()
	conf.SelectedCluster = ""
	err := conf.Persist()
	if err != nil {
		return err
	}
	fmt.Println("Active cluster has been reset.")
	return nil
}

/**
 * Configures cluster, including targets it should run
 */
func clusterConfigure(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	return ac.configureCluster()
}

func clusterGetToken(c *cli.Context) error {
	clusterName := c.String("cluster")
	cm, err := ClusterManagerForCluster(clusterName)
	if err != nil {
		return err
	}
	token, err := cm.KubernetesProvider().GetAuthToken(context.TODO(), clusterName)
	if err != nil {
		return err
	}

	result, _ := json.Marshal(token)
	fmt.Println(string(result))

	return nil
}

func getActiveCluster() (*activeCluster, error) {
	conf := config.GetConfig()
	if conf.SelectedCluster == "" {
		return nil, fmt.Errorf("Cluster not selected yet. Select one with 'kon cluster select ...'")
	}

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
	// load new resources into kube
	fmt.Println("Loading custom resource definitions into Kubernetes...")
	for _, file := range KUBE_RESOURCES {
		err := utils.KubeApplyFile(file)
		if err != nil {
			return errors.Wrapf(err, "Unable to apply config %s", file)
		}
	}

	err := utils.WaitUntilComplete(utils.ShortTimeoutSec, utils.MediumCheckInterval, func() (bool, error) {
		// use a new kclient to avoid caching
		contextName := resources.ContextNameForCluster(c.Manager.Cloud(), c.Cluster)
		kclient, err := KubernetesClientWithContext(contextName)
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
	if err = resources.SaveNodepool(kclient, np); err != nil {
		return err
	}

	fmt.Printf("Successfully created nodepool %s\n", np.GetObjectMeta().GetName())
	return nil
}

func (c *activeCluster) installComponents() error {
	kclient := c.kubernetesClient()
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}
	// now install all these resources
	installed := make(map[string]string)
	for _, comp := range cc.Status.InstalledComponents {
		installed[comp.Name] = comp.Version
	}

	for _, comp := range cc.Spec.Components {
		if installed[comp.Name] != "" {
			continue
		}
		compInstaller := components.GetComponentByName(comp.Name)
		if compInstaller == nil {
			return fmt.Errorf("Cluster requires %s, which is no longer available")
		}

		fmt.Printf("Installing Kubernetes components for %s\n", compInstaller.Name())

		// TODO: better handle versions
		if compInstaller.Version() != comp.Version {
			return fmt.Errorf("Version mismatch for %s: specified: %s, current: %s",
				compInstaller.Name(), comp.Version, compInstaller.Version())
		}

		err = compInstaller.InstallComponent(kclient)
		if err != nil {
			return err
		}

		// mark it as installed
		cc.Status.InstalledComponents = append(cc.Status.InstalledComponents, v1alpha1.ComponentSpec{
			Name:    compInstaller.Name(),
			Version: compInstaller.Version(),
		})
		err = kclient.Status().Update(context.Background(), cc)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *activeCluster) generateKubeConfig() error {
	// spec from: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins
	// find current executable path
	cmdPath, err := os.Executable()
	if err != nil {
		return err
	}
	cmdPath, err = filepath.Abs(cmdPath)
	if err != nil {
		return err
	}

	clusterConfs := []*resources.KubeClusterConfig{}
	selectedIdx := -1
	for _, cm := range GetClusterManagers() {
		ksvc := cm.KubernetesProvider()
		if ksvc == nil {
			continue
		}
		clusters, err := ksvc.ListClusters(context.Background())
		if err != nil {
			return err
		}
		for i, cluster := range clusters {
			cc := &resources.KubeClusterConfig{
				Cloud:       cm.Cloud(),
				Cluster:     cluster.Name,
				CAData:      []byte(cluster.CertificateAuthorityData),
				EndpointUrl: cluster.Endpoint,
			}
			clusterConfs = append(clusterConfs, cc)

			if c.Manager.Cloud() == cm.Cloud() && c.Cluster == cluster.Name {
				selectedIdx = i
			}
		}
	}

	// write to kube config
	target, err := kubeConfigPath()
	if err != nil {
		return err
	}

	if isExternalKubeConfig(target) {
		// already exists.. warn user
		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("%s already exists, overwrite", target),
			IsConfirm: true,
		}
		utils.FixPromptBell(&prompt)
		_, err = prompt.Run()
		if err != nil {
			// prompt aborted
			fmt.Printf("selecting a cluster requires writing to ~/.kube/config. To try this again run `%s cluster select --cluster %s`\n",
				config.ExecutableName, c.Cluster)
			return fmt.Errorf("select aborted")
		}
	}
	fmt.Printf("configuring kubectl: generating %s\n", target)

	kubeConf := resources.GenerateKubeConfig(cmdPath, clusterConfs, selectedIdx)
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer func() {
		if file != nil {
			file.Close()
		}
	}()

	serializer := k8sJson.NewYAMLSerializer(k8sJson.DefaultMetaFactory, nil, nil)
	err = serializer.Encode(kubeConf, file)
	if err != nil {
		return err
	}
	file.Close()
	file = nil

	// generate checksum
	cp := checksumPath(target)
	checksum, err := files.Sha1ChecksumFile(target)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(cp, []byte(checksum), files.DefaultFileMode)
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
	kclient, err := KubernetesClientWithContext(resources.ContextNameForCluster(c.Manager.Cloud(), c.Cluster))
	if err != nil {
		return errors.Wrap(err, "Unable to create Kubernetes Client")
	}
	c.kclient = kclient
	return nil
}

func printClusterSection(section providers.ClusterManager, clusters []*clusterInfo) {
	fmt.Printf("\nCloud: %v\n", section)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Cluster", "Version", "Status", "Konstellation", "Targets", "Provider ID"})
	for _, ci := range clusters {
		c := ci.Cluster
		targets := []string{}
		konVersion := ""
		if ci.Config != nil {
			targets = ci.Config.Spec.Targets
			konVersion = ci.Config.Spec.Version
			if konVersion != version.Version {
				konVersion = fmt.Sprintf("%s (current %s)", konVersion, version.Version)
			}
		} else {
			if c.Status == types.StatusActive {
				c.Status = types.StatusUnconfigured
			}
		}
		table.Append([]string{
			c.Name,
			c.Version,
			c.Status.String(),
			konVersion,
			strings.Join(targets, ","),
			c.ID,
		})
	}
	utils.FormatTable(table)
	table.Render()
}

func loadExistingConfigs(ccPath, npPath string) (cc *v1alpha1.ClusterConfig, np *v1alpha1.Nodepool, err error) {
	cc = &v1alpha1.ClusterConfig{}
	np = &v1alpha1.Nodepool{}
	ccObj, err := utils.LoadKubeObject(GetKubeDecoder(), cc, ccPath)
	if err != nil {
		return
	}
	cc, ok := ccObj.(*v1alpha1.ClusterConfig)
	if !ok {
		err = fmt.Errorf("type mismatch")
		return
	}

	npObj, err := utils.LoadKubeObject(GetKubeDecoder(), np, npPath)
	if err != nil {
		return
	}
	np, ok = npObj.(*v1alpha1.Nodepool)
	if !ok {
		err = fmt.Errorf("type mismatch")
		return
	}
	return
}

func isExternalKubeConfig(configPath string) bool {
	if _, err := os.Stat(configPath); err == nil {
		// see if it's already managed by konstellation
		cp := checksumPath(configPath)
		if _, err = os.Stat(cp); err == nil {
			// check checksum is the same
			existingSha, err := files.Sha1ChecksumFile(configPath)
			if err != nil {
				return true
			}
			checksumContent, err := ioutil.ReadFile(cp)
			if err != nil {
				return true
			}
			return !bytes.Equal([]byte(existingSha), checksumContent)
		} else {
			// no checksum file, unrelated kubeconfig
			return true
		}
	}
	return false
}

func checksumPath(configPath string) string {
	configDir := path.Dir(configPath)
	configName := path.Base(configPath)
	return path.Join(configDir, fmt.Sprintf(".%s.konsha", configName))
}

func updateClusterLocations() error {
	conf := config.GetConfig()
	conf.Clusters = make(map[string]*config.ClusterLocation)

	for _, cm := range GetClusterManagers() {
		ksvc := cm.KubernetesProvider()
		if ksvc == nil {
			continue
		}
		clusters, err := ksvc.ListClusters(context.Background())
		if err != nil {
			return err
		}

		for _, cluster := range clusters {
			conf.Clusters[cluster.Name] = &config.ClusterLocation{
				Cloud:  cm.Cloud(),
				Region: cm.Region(),
			}
		}
	}
	return conf.Persist()
}
