package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/davidzhao/konstellation/cmd/kon/config"
	"github.com/davidzhao/konstellation/cmd/kon/providers"
	"github.com/davidzhao/konstellation/cmd/kon/templates"
	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/apis"
	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/cloud/types"
	"github.com/davidzhao/konstellation/pkg/resources"
	"github.com/davidzhao/konstellation/pkg/utils/files"
	"github.com/davidzhao/konstellation/version"
	"github.com/manifoldco/promptui"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kconf "sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	clusterCloudFlag = cli.StringFlag{
		Name:  "cloud",
		Usage: "the cloud that the cluster resides",
		Value: "aws",
		// Required: true,
	}
	clusterNameFlag = cli.StringFlag{
		Name:     "cluster",
		Usage:    "cluster name",
		Required: true,
	}
)

var ClusterCommands = []cli.Command{
	cli.Command{
		Name:  "cluster",
		Usage: "Kubernetes cluster management",
		Subcommands: []cli.Command{
			cli.Command{
				Name:   "list",
				Usage:  "list clusters",
				Action: clusterList,
			},
			cli.Command{
				Name:   "create",
				Usage:  "creates a cluster",
				Action: clusterCreate,
			},
			cli.Command{
				Name:   "select",
				Usage:  "select an active cluster to work with",
				Action: clusterSelect,
				Flags: []cli.Flag{
					clusterCloudFlag,
					clusterNameFlag,
				},
			},
			cli.Command{
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

func clusterList(c *cli.Context) error {
	for _, c := range AvailableClouds {
		ksvc := c.KubernetesProvider()
		if ksvc == nil {
			continue
		}
		clusters, err := ksvc.ListClusters(context.Background())
		if err != nil {
			return err
		}
		printClusterSection(c, clusters)
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

	err := generateKubeConfig(cloud, conf.SelectedCluster)
	if err != nil {
		return err
	}

	err = conf.Persist()
	if err != nil {
		return err
	}

	err = configureCluster(cloud, conf.SelectedCluster)
	if err != nil {
		return err
	}

	// configure nodepool & cluster
	return configureNodepool(cloud, conf.SelectedCluster)
}

func configureCluster(cloud providers.CloudProvider, clusterName string) error {
	kclient, err := KubernetesClient()
	if err != nil {
		return err
	}
	// get cluster config and status, if all of the components are installed, then we are good to go
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		// create initial config
		cc = &v1alpha1.ClusterConfig{
			Spec: v1alpha1.ClusterConfigSpec{
				Version: version.Version,
			},
		}
		cc.ObjectMeta.Name = clusterName
		for _, comp := range config.Components {
			cc.Spec.Components = append(cc.Spec.Components, v1alpha1.ClusterComponent{
				Name:    comp.Name(),
				Version: comp.Version(),
			})
		}
		err = kclient.Create(context.Background(), cc)
		if err != nil {
			return err
		}

		// load new resources into kube
		for _, file := range KUBE_RESOURCES {
			err := utils.KubeApply(file)
			if err != nil {
				return errors.Wrapf(err, "Unable to apply config %s", file)
			}
		}
	}

	// now install all these resources
	installed := make(map[string]bool)
	for _, comp := range cc.Status.InstalledComponents {
		installed[comp] = true
	}

	for _, comp := range config.Components {
		if installed[comp.Name()] {
			continue
		}
		if comp.NeedsCLI() {
			err = comp.InstallCLI()
			if err != nil {
				return err
			}
		}
		err = comp.InstallComponent(kclient)
		if err != nil {
			return err
		}

		// mark it as installed
		cc.Status.InstalledComponents = append(cc.Status.InstalledComponents, comp.Name())
		err = kclient.Status().Update(context.Background(), cc)
		if err != nil {
			return err
		}
	}

	return nil
}

// check nodepool status, if ready continue
func configureNodepool(cloud providers.CloudProvider, clusterName string) error {
	kclient, err := KubernetesClient()
	if err != nil {
		return err
	}
	np, err := resources.GetNodepoolOfType(kclient, resources.NODEPOOL_PRIMARY)

	if err != nil && np == nil {
		// set configuration for the first time
		np, err = cloud.ConfigureNodepool(clusterName)
		if err != nil {
			return err
		}
		np.SetLabels(map[string]string{
			resources.NODEPOOL_LABEL: resources.NODEPOOL_PRIMARY,
		})

		// save spec to Kube
		err = kclient.Create(context.Background(), np)
		if err != nil {
			return err
		}
	}

	// if kube status is already reconciled, we can skip
	if np.Status.NumReady > 0 {
		return nil
	}

	// check aws nodepool status. if it doesn't exist, then create it
	kubeProvider := cloud.KubernetesProvider()
	ready, err := kubeProvider.IsNodepoolReady(context.Background(),
		clusterName, np.GetObjectMeta().GetName())
	if err != nil {
		// nodepool doesn't exist, create it
		err = kubeProvider.CreateNodepool(context.Background(), clusterName, np, resources.NODEPOOL_PRIMARY)
		if err != nil {
			return err
		}
	}

	// wait for completion
	if !ready {
		fmt.Printf("Waiting for nodepool become ready, this may take a few minutes\n")
		err = utils.WaitUntilComplete(utils.LongTimeoutSec, utils.LongCheckInterval, func() (bool, error) {
			return kubeProvider.IsNodepoolReady(context.Background(),
				clusterName, np.GetObjectMeta().GetName())
		})
		if err != nil {
			return err
		}
	}

	err = resources.UpdateStatus(kclient, np)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully created nodepool %s\n", np.GetObjectMeta().GetName())
	return nil
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

	result, err := json.Marshal(token)
	fmt.Println(string(result))

	return nil
}

func printClusterSection(section providers.CloudProvider, clusters []*types.Cluster) {
	fmt.Printf("\nCloud: %s\n", section.ID())

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Cluster", "Version", "Status", "Provider ID"})
	for _, c := range clusters {
		table.Append([]string{c.Name, c.Version, c.Status.String(), c.ID})
	}
	utils.FormatTable(table)
	table.Render()
}

func generateKubeConfig(cloud providers.CloudProvider, clusterName string) error {
	// spec from: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins
	if !cloud.IsSetup() {
		return fmt.Errorf("%s has not been setup", cloud)
	}
	cluster, err := cloud.KubernetesProvider().GetCluster(context.Background(), clusterName)
	if err != nil {
		return err
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
	tokenArgs := []string{
		"cluster",
		"get-token",
		"--cloud",
		cloud.ID(),
		"--cluster",
		clusterName,
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
				config.ExecutableName, cloud.ID(), clusterName)
			return fmt.Errorf("select aborted")
		}
	}
	fmt.Printf("configuring kubectl: generating %s\n", target)

	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()
	configContent := templates.KubeConfig(cluster.Endpoint, cluster.CertificateAuthorityData, cmdPath, tokenArgs)
	_, err = file.WriteString(configContent)
	if err != nil {
		return err
	}

	// generate checksum
	cp := checksumPath(target)
	return ioutil.WriteFile(cp, []byte(files.Sha1ChecksumString(configContent)), files.DefaultFileMode)
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

func KubernetesClient() (client.Client, error) {
	// construct a client from local config
	scheme := runtime.NewScheme()
	// register both our scheme and konstellation scheme
	clientgoscheme.AddToScheme(scheme)
	apis.AddToScheme(scheme)
	conf, err := kconf.GetConfig()
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
