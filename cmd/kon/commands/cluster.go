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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sJson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kconf "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/providers"
	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/apis"
	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/cloud/types"
	"github.com/davidzhao/konstellation/pkg/resources"
	"github.com/davidzhao/konstellation/pkg/utils/files"
	"github.com/davidzhao/konstellation/pkg/utils/objects"
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
	&cli.Command{
		Name:  "cluster",
		Usage: "Kubernetes cluster management",
		Subcommands: []*cli.Command{
			&cli.Command{
				Name:   "list",
				Usage:  "list clusters",
				Action: clusterList,
			},
			&cli.Command{
				Name:   "create",
				Usage:  "creates a cluster",
				Action: clusterCreate,
			},
			&cli.Command{
				Name:   "select",
				Usage:  "select an active cluster to work with",
				Action: clusterSelect,
				Flags: []cli.Flag{
					clusterCloudFlag,
					clusterNameFlag,
				},
			},
			&cli.Command{
				Name:   "configure",
				Usage:  "configure cluster settings",
				Action: clusterConfigure,
			},
			&cli.Command{
				Name:   "get-token",
				Usage:  "returns a kubernetes compatible token",
				Action: clusterGetToken,
				Flags: []cli.Flag{
					clusterCloudFlag,
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
		fmt.Printf("\nSelected cluster %s (%s)\n", conf.SelectedCluster, conf.SelectedCloud)
	}
	for _, c := range AvailableClouds {
		ksvc := c.KubernetesProvider()
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

			contextName := resources.ContextNameForCluster(c.ID(), cluster.Name)
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
		printClusterSection(c, infos)
	}

	return nil
}

func clusterCreate(c *cli.Context) error {
	fmt.Println(CLUSTER_CREATE_HELP)
	cloud, err := ChooseCloudPrompt("")
	if err != nil {
		return err
	}
	name, err := cloud.CreateCluster()
	if err != nil {
		return err
	}
	fmt.Printf("Successfully created cluster %s. Waiting for cluster to become ready\n", name)
	err = utils.WaitUntilComplete(utils.LongTimeoutSec, utils.LongCheckInterval, func() (bool, error) {
		cluster, err := cloud.KubernetesProvider().GetCluster(context.Background(), name)
		if err != nil {
			return false, err
		}
		if cluster.Status == types.StatusCreating {
			return false, nil
		} else if cluster.Status == types.StatusActive {
			return true, nil
		} else {
			return false, fmt.Errorf("Unexpected cluster status during creation: %s", cluster.Status)
		}
	})

	if err == context.DeadlineExceeded {
		// couldn't verify after waiting
	} else if err != nil {
		return err
	}

	c.Set("cloud", cloud.ID())
	c.Set("cluster", name)

	return clusterSelect(c)
}

func clusterSelect(c *cli.Context) error {
	conf := config.GetConfig()
	conf.SelectedCloud = c.String("cloud")
	conf.SelectedCluster = c.String("cluster")

	cloud := CloudProviderByID(c.String("cloud"))
	if cloud == nil {
		return nil
	}
	ac := activeCluster{
		Cloud:   cloud,
		Cluster: conf.SelectedCluster,
	}

	kubeProvider := cloud.KubernetesProvider()
	cluster, err := kubeProvider.GetCluster(context.Background(), conf.SelectedCluster)
	if err != nil {
		return err
	}

	if cluster.Status == types.StatusCreating {
		fmt.Println("Waiting for cluster to become ready")
		err = utils.WaitUntilComplete(utils.LongTimeoutSec, utils.LongCheckInterval, func() (bool, error) {
			cluster, err := kubeProvider.GetCluster(context.Background(), conf.SelectedCluster)
			if err != nil {
				return false, err
			}
			if cluster.Status == types.StatusCreating {
				return false, nil
			} else if cluster.Status == types.StatusActive {
				return true, nil
			} else {
				return false, fmt.Errorf("Unexpected cluster status: %s", cluster.Status)
			}
		})
		if err != nil {
			return err
		}
	} else if cluster.Status != types.StatusActive {
		return fmt.Errorf("Cannot select cluster, status: %s", cluster.Status.String())
	}

	err = ac.generateKubeConfig()
	if err != nil {
		return err
	}

	err = conf.Persist()
	if err != nil {
		return err
	}

	kclient := ac.kubernetesClient()
	// see if we have to configure cluster
	_, err = resources.GetClusterConfig(kclient)
	if err != nil {
		// have not been configured, do it
		err = ac.createClusterConfig()
		if err != nil {
			return err
		}
		err = ac.configureCluster()

	} else {
		// TODO: in release versions don't reload resources
		// still load the resources
		err = ac.loadResourcesIntoKube()
	}
	if err != nil {
		return err
	}

	// see if we have a nodepool
	_, err = resources.GetNodepoolOfType(kclient, resources.NODEPOOL_PRIMARY)
	if err != nil {
		err = ac.configureNodepool()
	}
	if err != nil {
		return err
	}

	return ac.installComponents()
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
	cloud := CloudProviderByID(c.String("cloud"))
	if cloud == nil {
		return nil
	}
	token, err := cloud.KubernetesProvider().GetAuthToken(context.Background(), c.String("cluster"))
	if err != nil {
		return err
	}

	result, _ := json.Marshal(token)
	fmt.Println(string(result))

	return nil
}

func getActiveCluster() (*activeCluster, error) {
	conf := config.GetConfig()
	if conf.SelectedCloud == "" || conf.SelectedCluster == "" {
		return nil, fmt.Errorf("Cluster not selected yet. Select one with 'kon cluster select ...'")
	}

	cloud := CloudProviderByID(conf.SelectedCloud)
	if cloud == nil {
		return nil, fmt.Errorf("Unable to find cloud provider for %s", conf.SelectedCloud)
	}

	ac := activeCluster{
		Cloud:   cloud,
		Cluster: conf.SelectedCluster,
	}

	err := ac.initClient()
	if err != nil {
		return nil, err
	}
	return &ac, nil
}

type activeCluster struct {
	Cloud   providers.CloudProvider
	Cluster string
	kclient client.Client
}

func (c *activeCluster) createClusterConfig() error {
	kclient := c.kubernetesClient()

	// load new resources into kube
	err := c.loadResourcesIntoKube()
	if err != nil {
		return err
	}

	err = utils.WaitUntilComplete(utils.ShortTimeoutSec, utils.MediumCheckInterval, func() (bool, error) {
		_, err := resources.GetClusterConfig(kclient)
		if err != resources.ErrNotFound {
			return false, nil
		} else {
			return true, nil
		}
	})
	if err != nil {
		return err
	}

	// create initial config
	cc := &v1alpha1.ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.Cluster,
		},
	}
	_, err = controllerutil.CreateOrUpdate(context.TODO(), kclient, cc, func() error {
		cc.Spec = v1alpha1.ClusterConfigSpec{
			Version: version.Version,
		}
		for _, comp := range config.Components {
			cc.Spec.Components = append(cc.Spec.Components, v1alpha1.ClusterComponent{
				ComponentSpec: v1alpha1.ComponentSpec{
					Name:    comp.Name(),
					Version: comp.Version(),
				},
			})
		}
		return nil
	})
	return err
}

func (c *activeCluster) loadResourcesIntoKube() error {
	// load new resources into kube
	fmt.Println("Loading Konstellation resources")
	for _, file := range KUBE_RESOURCES {
		err := utils.KubeApplyFile(file)
		if err != nil {
			return errors.Wrapf(err, "Unable to apply config %s", file)
		}
	}
	return nil
}

// check nodepool status, if ready continue
func (c *activeCluster) configureNodepool() error {
	kclient := c.kubernetesClient()

	// set configuration for the first time
	np, err := c.Cloud.ConfigureNodepool(c.Cluster)
	if err != nil {
		return err
	}
	np.SetLabels(map[string]string{
		resources.NODEPOOL_LABEL: resources.NODEPOOL_PRIMARY,
	})

	// save spec to Kube
	existing := &v1alpha1.Nodepool{
		ObjectMeta: metav1.ObjectMeta{
			Name: np.Name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(context.TODO(), kclient, existing, func() error {
		objects.MergeObject(&existing.Spec, &np.Spec)
		return nil
	})
	if err != nil {
		return err
	}

	// if kube status is already reconciled, we can skip
	if existing.Status.NumReady > 0 {
		return nil
	}

	fmt.Println("Creating nodepool...")

	// check aws nodepool status. if it doesn't exist, then create it
	kubeProvider := c.Cloud.KubernetesProvider()
	ready, err := kubeProvider.IsNodepoolReady(context.Background(), c.Cluster, np.Name)
	if err != nil {
		// nodepool doesn't exist, create it
		err = kubeProvider.CreateNodepool(context.Background(), c.Cluster, np, resources.NODEPOOL_PRIMARY)
		if err != nil {
			return err
		}
	}

	// wait for completion
	if !ready {
		fmt.Printf("Waiting for nodepool become ready, this may take a few minutes\n")
		err = utils.WaitUntilComplete(utils.LongTimeoutSec, utils.LongCheckInterval, func() (bool, error) {
			return kubeProvider.IsNodepoolReady(context.Background(), c.Cluster, existing.Name)
		})
		if err != nil {
			return err
		}
	}

	err = resources.UpdateStatus(kclient, existing)
	if err != nil {
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

	for _, comp := range config.Components {
		if installed[comp.Name()] != "" {
			continue
		}
		fmt.Printf("Installing Kubernetes components for %s\n", comp.Name())
		err = comp.InstallComponent(kclient)
		if err != nil {
			return err
		}

		// mark it as installed
		cc.Status.InstalledComponents = append(cc.Status.InstalledComponents, v1alpha1.ComponentSpec{
			Name:    comp.Name(),
			Version: comp.Version(),
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
	if !c.Cloud.IsSetup() {
		return fmt.Errorf("%s has not been setup", c.Cloud)
	}
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
	for _, cloud := range AvailableClouds {
		ksvc := cloud.KubernetesProvider()
		if ksvc == nil {
			continue
		}
		clusters, err := ksvc.ListClusters(context.Background())
		if err != nil {
			return err
		}
		for i, cluster := range clusters {
			cc := &resources.KubeClusterConfig{
				Cloud:       cloud.ID(),
				Cluster:     cluster.Name,
				CAData:      []byte(cluster.CertificateAuthorityData),
				EndpointUrl: cluster.Endpoint,
			}
			clusterConfs = append(clusterConfs, cc)

			if cloud.ID() == c.Cloud.ID() && c.Cluster == cluster.Name {
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
		_, err = prompt.Run()
		if err != nil {
			// prompt aborted
			fmt.Printf("selecting a cluster requires writing to ~/.kube/config. To try this again run `%s cluster select --cloud %s --cluster %s`\n",
				config.ExecutableName, c.Cloud.ID(), c.Cluster)
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
	kclient, err := KubernetesClientWithContext(resources.ContextNameForCluster(c.Cloud.ID(), c.Cluster))
	if err != nil {
		return errors.Wrap(err, "Unable to create Kubernetes Client")
	}
	c.kclient = kclient
	return nil
}

func printClusterSection(section providers.CloudProvider, clusters []*clusterInfo) {
	fmt.Printf("\nCloud: %s\n", section.ID())

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

func KubernetesClientWithContext(contextName string) (client.Client, error) {
	// construct a client from local config
	scheme := runtime.NewScheme()
	// register both our scheme and konstellation scheme
	clientgoscheme.AddToScheme(scheme)
	apis.AddToScheme(scheme)
	conf, err := kconf.GetConfigWithContext(contextName)
	if err != nil {
		return nil, err
	}
	return client.New(conf, client.Options{Scheme: scheme})
}

func kubeConfigPath() (string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return path.Join(homedir, ".kube", "config"), nil
}
