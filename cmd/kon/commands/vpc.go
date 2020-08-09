package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cast"
	"github.com/urfave/cli/v2"

	"github.com/k11n/konstellation/cmd/kon/providers"
	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/cloud/types"
)

var VPCCommands = []*cli.Command{
	{
		Name:     "vpc",
		Usage:    "VPC management",
		Category: "Other",
		Before:   ensureSetup,
		Subcommands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "list VPCs in configured regions",
				Action: vpcList,
			},
			{
				Name:   "destroy",
				Usage:  "destroys a Konstellation-created VPC",
				Action: vpcDestroy,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "vpc",
						Usage:    "ID of VPC to destroy",
						Required: true,
					},
				},
			},
		},
	},
}

func vpcList(c *cli.Context) error {
	for _, cm := range GetClusterManagers() {
		fmt.Printf("\n%s (%s)\n", cm.Cloud(), cm.Region())

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"VPC", "CIDR Block", "Konstellation", "Topology", "IPv6"})
		utils.FormatTable(table)

		vpcs, err := cm.VPCProvider().ListVPCs(context.Background())
		if err != nil {
			return err
		}

		for _, vpc := range vpcs {
			supportsKonstellation := "no"
			if vpc.SupportsKonstellation {
				supportsKonstellation = "yes"
			}
			table.Append([]string{
				vpc.ID,
				vpc.CIDRBlock,
				supportsKonstellation,
				string(vpc.Topology),
				cast.ToString(vpc.IPv6),
			})
		}

		table.Render()
	}
	return nil
}

func vpcDestroy(c *cli.Context) error {
	vpcId := c.String("vpc")

	var vpc *types.VPC
	var err error
	var manager providers.ClusterManager
	for _, cm := range GetClusterManagers() {
		vpc, err = cm.VPCProvider().GetVPC(context.Background(), vpcId)
		if err != nil {
			continue
		}
		if vpc != nil {
			manager = cm
			break
		}
	}

	if manager == nil {
		return fmt.Errorf("unsupported VPC")
	}

	if vpc == nil {
		return fmt.Errorf("could not find VPC %s", vpcId)
	}

	if !vpc.SupportsKonstellation {
		return fmt.Errorf("cannot destroy VPC, not created by Konstellation")
	}

	// ask for confirmation
	err = utils.ExplicitConfirmationPrompt(
		fmt.Sprintf("Do you want to delete VPC %s in %s/%s", vpc.ID, manager.Cloud(), manager.Region()),
		vpc.ID)
	if err != nil {
		return err
	}

	if err = manager.DestroyVPC(vpc.ID); err != nil {
		return err
	}

	fmt.Println("Successfully destroyed VPC", vpc.ID)
	return nil
}
