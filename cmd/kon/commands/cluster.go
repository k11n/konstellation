package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	cliv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	metrics "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/cmd/kon/config"
	"github.com/k11n/konstellation/cmd/kon/kube"
	"github.com/k11n/konstellation/cmd/kon/providers"
	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/cloud/types"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/pkg/utils/files"
	"github.com/k11n/konstellation/version"
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
		Name:     "cluster",
		Usage:    "Cluster commands",
		Before:   ensureSetup,
		Category: "Cluster",
		Subcommands: []*cli.Command{
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
				Name:   "export",
				Usage:  "export apps, configs, and other cluster data",
				Action: clusterExport,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "dir",
						Usage:    "directory to export cluster data to",
						Required: true,
					},
				},
			},
			{
				Name:   "import",
				Usage:  "import apps, builds, and configs into the selected cluster",
				Action: clusterImport,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "dir",
						Usage:    "directory to import cluster data from",
						Required: true,
					},
				},
			},
			{
				Name:   "list",
				Usage:  "list clusters",
				Action: clusterList,
			},
			{
				Name:      "select",
				Usage:     "select an active cluster to work with",
				ArgsUsage: "<cluster>",
				Action: func(c *cli.Context) error {
					if c.Bool("reset") {
						return clusterReset()
					}
					if c.NArg() == 0 {
						cli.ShowSubcommandHelp(c)
						return nil
					}
					return clusterSelect(c.Args().Get(0))
				},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "reset",
						Usage: "unset current selected cluster",
					},
				},
			},
			{
				Name:   "shell",
				Usage:  "shell into a pod created for debugging. (deleted after terminating)",
				Action: clusterShell,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "image",
						Usage: "docker image to use for the pod",
						Value: "debian:latest",
					},
					&cli.BoolFlag{
						Name:  "force",
						Usage: "deletes an existing debugging image if exists",
					},
				},
			},
			{
				Name:   "reinstall",
				Usage:  "reinstalls Konstellation components",
				Action: clusterReinstall,
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
	Cluster     *types.Cluster
	Config      *v1alpha1.ClusterConfig
	Nodepools   []*v1alpha1.Nodepool
	Nodes       []corev1.Node
	NodeMetrics []metrics.NodeMetrics
}

func clusterList(c *cli.Context) error {
	conf := config.GetConfig()

	if err := updateClusterLocations(); err != nil {
		return err
	}
	if conf.IsClusterSelected() {
		if _, ok := conf.Clusters[conf.SelectedCluster]; ok {
			fmt.Printf("\nSelected cluster %s\n", conf.SelectedCluster)
		} else if err := clusterReset(); err != nil {
			return err
		}
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

		infos := make([]*clusterInfo, 0, len(clusters))
		var wg sync.WaitGroup
		for _, cluster := range clusters {
			info := &clusterInfo{
				Cluster: cluster,
			}
			infos = append(infos, info)

			contextName := resources.ContextNameForCluster(cm.Cloud(), cluster.Name)
			kclient, err := kube.KubernetesClientWithContext(contextName)
			if err != nil {
				continue
			}

			// get cluster config
			wg.Add(1)
			go func(kc client.Client, info2 *clusterInfo) {
				defer wg.Done()
				info2.Config, err = resources.GetClusterConfig(kc)
				if err != nil {
					fmt.Println("error getting cluster config", err)
				}
			}(kclient, info)

			// get nodepools
			wg.Add(1)
			go func(kc client.Client, info2 *clusterInfo) {
				defer wg.Done()
				info2.Nodepools, err = resources.GetNodepools(kc)
				if err != nil {
					fmt.Println("error getting modepools", err)
				}
			}(kclient, info)

			// get all nodes
			wg.Add(1)
			go func(kc client.Client, info2 *clusterInfo) {
				defer wg.Done()
				nodeList := corev1.NodeList{}
				if err = kc.List(context.TODO(), &nodeList); err == nil {
					info2.Nodes = nodeList.Items
				} else {
					fmt.Println("error getting node list", err)
				}
			}(kclient, info)

			// get node metrics
			wg.Add(1)
			go func(kc client.Client, info2 *clusterInfo) {
				wg.Done()
				metricsList := metrics.NodeMetricsList{}
				if err = kc.List(context.TODO(), &metricsList); err == nil {
					info2.NodeMetrics = metricsList.Items
				} else {
					fmt.Println("error getting node metrics", err)
				}
			}(kclient, info)
		}

		wg.Wait()
		if len(infos) > 0 {
			printClusterSection(cm, infos)
		}
	}

	return nil
}

const clusterCreateHelp = `Creates a new Konstellation Kubernetes cluster.`

func clusterCreate(c *cli.Context) error {
	if !config.GetConfig().IsSetup() {
		return fmt.Errorf("Konstellation has not been setup yet. Run `kon setup`")
	}
	fmt.Println(clusterCreateHelp)

	if err := installBundledCli(); err != nil {
		return err
	}

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
			err = cm.CheckCreatePermissions()
			if err != nil {
				return err
			}
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

		err = cm.CheckCreatePermissions()
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
		err = utils.SaveKubeObject(kube.GetKubeEncoder(), cc, clusterConfigFile)
		if err != nil {
			return err
		}

		nodepool, err = generator.CreateNodepoolConfig(cc)
		if err != nil {
			return err
		}
		err = utils.SaveKubeObject(kube.GetKubeEncoder(), nodepool, nodepoolConfigFile)
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
	if err = generateKubeConfig(); err != nil {
		return err
	}

	if err = ac.loadResourcesIntoKube(); err != nil {
		return err
	}
	if _, err = resources.UpdateResource(ac.kubernetesClient(), cc, nil, nil); err != nil {
		return err
	}
	if _, err = resources.UpdateResource(ac.kubernetesClient(), nodepool, nil, nil); err != nil {
		return err
	}

	// activate
	if err = cm.ActivateCluster(cc); err != nil {
		return err
	}

	// delete state files to clear up half completed state
	os.Remove(clusterConfigFile)
	os.Remove(nodepoolConfigFile)

	fmt.Println()
	fmt.Printf("Cluster %s has been successfully created. Run `kon cluster select %s` to use it\n", cc.Name, cc.Name)
	return nil
}

func clusterExport(c *cli.Context) error {
	target := c.String("dir")
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	exporter := resources.NewExporter(ac.kubernetesClient(), target)
	err = exporter.Export()
	if err != nil {
		return err
	}

	fmt.Println("Successfully exported cluster to", target)
	return nil
}

func clusterImport(c *cli.Context) error {
	source := c.String("dir")
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	kclient := ac.kubernetesClient()

	importer := resources.NewImporter(kclient, source)
	err = importer.Import()
	if err != nil {
		return err
	}

	// reconcile linked accounts
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}
	if err = reconcileAccounts(ac, cc.Spec.Targets); err != nil {
		return err
	}

	fmt.Println("Successfully imported settings into cluster")
	return nil
}

func clusterDestroy(c *cli.Context) error {
	clusterName := c.String("cluster")
	// update clusters
	if err := updateClusterLocations(); err != nil {
		return err
	}
	cm, err := ClusterManagerForCluster(clusterName)
	if err != nil {
		return err
	}

	err = cm.CheckDestroyPermissions()
	if err != nil {
		return err
	}

	fmt.Printf("This will destroy the cluster %s, removing all of apps and configs on this cluster. This action cannot be reversed.\n", clusterName)

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

	// Reset it from being selected
	conf := config.GetConfig()
	if conf.SelectedCluster == clusterName {
		conf.SelectedCluster = ""
		err := conf.Persist()
		if err != nil {
			return err
		}
	}

	// remove all apps and then ingress
	// cluster might already be destroyed by then, so ignore these errors
	if kclient, err := kube.KubernetesClientWithContext(resources.ContextNameForCluster(cm.Cloud(), clusterName)); err == nil {
		apps, err := resources.ListApps(kclient)
		if err != nil {
			return err
		}

		for _, app := range apps {
			kclient.Delete(context.TODO(), &app)
		}

		// delete all ingresses
		ingress, err := resources.GetKonIngress(kclient)
		if err == nil {
			kclient.Delete(context.TODO(), ingress)
		}

		// delete all linked accounts
		resources.ForEach(kclient, &v1alpha1.LinkedServiceAccountList{}, func(obj interface{}) error {
			lsa := obj.(v1alpha1.LinkedServiceAccount)
			fmt.Println("Cleaning up LinkedServiceAccount", lsa.Name)
			cm.DeleteLinkedServiceAccount(clusterName, &lsa)
			return nil
		})
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
	conf := config.GetConfig()
	conf.SelectedCluster = clusterName
	err = generateKubeConfig()
	if err != nil {
		return err
	}

	kclient := ac.kubernetesClient()
	// see if we have to configure cluster
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}

	// only persist once connected
	err = conf.Persist()
	if err != nil {
		return err
	}

	// TODO: still load the resources in testing
	//err = ac.loadResourcesIntoKube()
	//if err != nil {
	//	return err
	//}

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

	// see if targets are set
	if len(cc.Spec.Targets) == 0 {
		if err := ac.configureCluster(); err != nil {
			return err
		}
	}

	if err = ac.installComponents(false); err != nil {
		return err
	}
	fmt.Println("Switched active cluster to", clusterName)
	return nil
}

func clusterShell(c *cli.Context) error {
	image := c.String("image")
	force := c.Bool("force")
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}
	kclient := ac.kubernetesClient()

	usr, err := user.Current()
	if err != nil {
		return err
	}

	podName := fmt.Sprintf("kon-debug-%s", usr.Username)
	pod := &corev1.Pod{}
	err = kclient.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: "default"}, pod)

	if err == nil {
		if force {
			if err = kclient.Delete(context.Background(), pod); err != nil {
				return err
			}
			// TODO: this is ugly, should wait till deletion confirmed
			time.Sleep(5 * time.Second)
		} else {
			fmt.Printf("Debugging pod %s already exists. Use --force to delete it and create a new one\n", podName)
			return nil
		}
	} else if client.IgnoreNotFound(err) != nil {
		return err
	}

	// not found, create
	fmt.Println("Creating new debugging pod", podName)
	cmd := exec.Command("kubectl", "run", "--rm", "-it", podName, "--image", image, "--restart=Never")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func clusterReset() error {
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

	conf := config.GetConfig()
	var status types.ClusterStatus
	if conf.Clusters[clusterName] != nil {
		status = conf.Clusters[clusterName].Status
	}

	token, err := cm.KubernetesProvider().GetAuthToken(context.TODO(), clusterName, status)
	if err != nil {
		return err
	}

	result, _ := json.Marshal(token)
	fmt.Println(string(result))

	return nil
}

func clusterReinstall(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	// print warning
	fmt.Println("Reinstalling all Konstellation components onto the current cluster. This is NOT recommended as newer versions of components may be incompatible with your cluster.")
	if err := utils.ExplicitConfirmationPrompt("Sure you want to proceed?"); err != nil {
		return err
	}

	// reinstall configs
	if err = ac.loadResourcesIntoKube(); err != nil {
		return err
	}

	if err = ac.installComponents(true); err != nil {
		return err
	}

	// set operator version
	kclient := ac.kubernetesClient()
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}
	cc.Spec.Version = version.Version
	if _, err = resources.UpdateResource(kclient, cc, nil, nil); err != nil {
		return err
	}

	fmt.Println("Successfully reinstalled Konstellation")
	return nil
}

func ensureClusterSelected() error {
	conf := config.GetConfig()
	if conf.SelectedCluster == "" {
		return fmt.Errorf("Cluster not selected yet. Select one with 'kon cluster select ...'")
	}
	return nil
}

func printClusterSection(section providers.ClusterManager, clusters []*clusterInfo) {
	fmt.Printf("\n%s (%s)\n", section.Cloud(), section.Region())

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Cluster", "Version", "Status", "Konstellation", "Targets", "Nodes", "CPU", "Memory"})
	for _, ci := range clusters {
		c := ci.Cluster
		targets := make([]string, 0)
		konVersion := ""
		if ci.Config != nil {
			if len(ci.Config.Status.InstalledComponents) == 0 || len(ci.Config.Spec.Targets) == 0 {
				c.Status = types.StatusUnconfigured
			}
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

		var nodeStr, cpuStr, memoryStr string

		maxNodes := int64(0)
		for _, np := range ci.Nodepools {
			maxNodes += np.Spec.MaxSize
		}
		if maxNodes > 0 {
			nodeStr = fmt.Sprintf("%d (max %d)", len(ci.Nodes), maxNodes)
		}

		var cpuTotal, memoryTotal resource.Quantity
		var cpuUsed, memoryUsed resource.Quantity

		for _, node := range ci.Nodes {
			cpuTotal.Add(*node.Status.Allocatable.Cpu())
			memoryTotal.Add(*node.Status.Allocatable.Memory())
		}
		for _, nm := range ci.NodeMetrics {
			cpuUsed.Add(*nm.Usage.Cpu())
			memoryUsed.Add(*nm.Usage.Memory())
		}
		if cpuTotal.Value() != 0 {
			cpuStr = fmt.Sprintf("%d/%d (%d%%)", cpuUsed.Value(), cpuTotal.Value(), cpuUsed.Value()*100/cpuTotal.Value())
		}
		if memoryTotal.Value() != 0 {
			mb := int64(1000 * 1000)
			memoryStr = fmt.Sprintf("%d/%dMi (%d%%)", memoryUsed.Value()/mb, memoryTotal.Value()/mb, memoryUsed.Value()*100/memoryTotal.Value())
		}

		// node metrics
		table.Append([]string{
			c.Name,
			c.Version,
			c.Status.String(),
			konVersion,
			strings.Join(targets, ","),
			nodeStr,
			cpuStr,
			memoryStr,
		})
	}
	utils.FormatTable(table)
	table.Render()
}

func loadExistingConfigs(ccPath, npPath string) (cc *v1alpha1.ClusterConfig, np *v1alpha1.Nodepool, err error) {
	cc = &v1alpha1.ClusterConfig{}
	np = &v1alpha1.Nodepool{}
	ccObj, err := utils.LoadKubeObject(kube.GetKubeDecoder(), cc, ccPath)
	if err != nil {
		return
	}
	cc, ok := ccObj.(*v1alpha1.ClusterConfig)
	if !ok {
		err = fmt.Errorf("type mismatch")
		return
	}

	npObj, err := utils.LoadKubeObject(kube.GetKubeDecoder(), np, npPath)
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

func updateClusterLocations() error {
	conf := config.GetConfig()
	conf.Clusters = make(map[string]*config.ClusterInfo)

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
			conf.Clusters[cluster.Name] = &config.ClusterInfo{
				Cloud:  cm.Cloud(),
				Region: cm.Region(),
				Status: cluster.Status,
			}
		}
	}
	return conf.Persist()
}

func generateKubeConfig() error {
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

	var selectedCluster, selectedCloud string
	if config.GetConfig().SelectedCluster != "" {
		selectedCluster = config.GetConfig().SelectedCluster
		cm, err := ClusterManagerForCluster(selectedCluster)
		if err != nil {
			return err
		}
		selectedCloud = cm.Cloud()
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
		for _, cluster := range clusters {
			cc := &resources.KubeClusterConfig{
				Cloud:       cm.Cloud(),
				Cluster:     cluster.Name,
				CAData:      []byte(cluster.CertificateAuthorityData),
				EndpointUrl: cluster.Endpoint,
			}
			clusterConfs = append(clusterConfs, cc)

			if selectedIdx == -1 || (selectedCluster == cluster.Name && selectedCloud == cm.Cloud()) {
				selectedIdx = len(clusterConfs) - 1
			}
		}
	}

	// write to kube config
	target, err := config.KubeConfigDir()
	if err != nil {
		return err
	}

	kconf := resources.NewKubeConfig()
	if _, err := os.Stat(target); err == nil {
		data, err := ioutil.ReadFile(target)
		if err != nil {
			return errors.Wrap(err, "Could not read existing kube config")
		}
		obj, _, err := kube.GetKubeDecoder().Decode(data, nil, &cliv1.Config{})
		if err != nil {
			return errors.Wrap(err, "Could not decode kube config")
		}
		kconf = obj.(*cliv1.Config)
	}
	if isExternalKubeConfig(target) {
		// already exists.. warn user
		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Konstellation will append new cluster info into Kube config at %s. Ok to continue", target),
			IsConfirm: true,
		}
		utils.FixPromptBell(&prompt)
		_, err = prompt.Run()
		if err != nil {
			// prompt aborted
			fmt.Println("Konstellation requires updating ~/.kube/config. Please try this again")
			return fmt.Errorf("select aborted")
		}
	}

	fmt.Printf("configuring kubectl: updating %s\n", target)
	resources.UpdateKubeConfig(kconf, cmdPath, clusterConfs, selectedIdx)
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer func() {
		if file != nil {
			file.Close()
		}
	}()

	err = kube.GetKubeEncoder().Encode(kconf, file)
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
