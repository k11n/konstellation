package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cast"
	"github.com/urfave/cli/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
)

var NodepoolCommands = []*cli.Command{
	{
		Name:     "nodepool",
		Usage:    "Nodepool management",
		Category: "Cluster",
		Before: func(c *cli.Context) error {
			return ensureClusterSelected()
		},
		Subcommands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "list Nodepools in the current cluster",
				Action: nodepoolList,
			},
			{
				Name:   "create",
				Usage:  "create a new Nodepool",
				Action: nodepoolCreate,
			},
			{
				Name:   "destroy",
				Usage:  "destroys a Nodepool",
				Action: nodepoolDestroy,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "nodepool",
						Usage:    "Name of Nodepool to destroy",
						Required: true,
					},
				},
			},
		},
	},
}

func nodepoolList(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	kclient := ac.kubernetesClient()
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{
		"Name",
		"Machine Type",
		"Num Nodes",
		"Min Size",
		"Max Size",
		"Disk Size",
		//"GPU",
	})

	err = resources.ForEach(kclient, &v1alpha1.NodepoolList{}, func(item interface{}) error {
		np := item.(v1alpha1.Nodepool)
		table.Append([]string{
			np.Name,
			np.Spec.MachineType,
			cast.ToString(np.Status.NumReady),
			cast.ToString(np.Spec.MinSize),
			cast.ToString(np.Spec.MaxSize),
			fmt.Sprintf("%d GiB", np.Spec.DiskSizeGiB),
			//cast.ToString(np.Spec.RequiresGPU),
		})
		return nil
	})
	if err != nil {
		return err
	}
	utils.FormatTable(table)
	table.Render()

	return nil
}

func nodepoolCreate(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	kclient := ac.kubernetesClient()
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}

	cloud := GetCloud(ac.Manager.Cloud())
	generator, err := PromptClusterGenerator(cloud, ac.Manager.Region())
	if err != nil {
		return err
	}

	nodepool, err := generator.CreateNodepoolConfig(cc)
	if err != nil {
		return err
	}

	// create it first to keep a record
	if _, err = resources.UpdateResource(ac.kubernetesClient(), nodepool, nil, nil); err != nil {
		return err
	}
	err = ac.Manager.CreateNodepool(cc, nodepool)
	if err != nil {
		return err
	}

	// save any modifications to nodepool
	if _, err = resources.UpdateResource(ac.kubernetesClient(), nodepool, nil, nil); err != nil {
		return err
	}

	fmt.Println("Successfully created nodepool", nodepool.Name)

	return nil
}

func nodepoolDestroy(c *cli.Context) error {
	npName := c.String("nodepool")

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	label := fmt.Sprintf("Sure you want to delete nodepool %s?", npName)
	if err := utils.ExplicitConfirmationPrompt(label); err != nil {
		return err
	}

	err = ac.Manager.DeleteNodepool(ac.Cluster, npName)
	if err != nil {
		return err
	}

	err = ac.kubernetesClient().Delete(context.Background(), &v1alpha1.Nodepool{
		ObjectMeta: metav1.ObjectMeta{Name: npName},
	})
	if err != nil {
		return err
	}

	fmt.Println("Deleted nodepool", npName)
	return nil
}
