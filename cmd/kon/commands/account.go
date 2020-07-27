package commands

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/cmd/kon/kube"
	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/resources"
	utilscli "github.com/k11n/konstellation/pkg/utils/cli"
)

var (
	accountFlag = &cli.StringFlag{
		Name:     "account",
		Usage:    "name of account",
		Required: true,
	}
)

var AccountCommands = []*cli.Command{
	{
		Name:        "account",
		Usage:       "Linked service accounts",
		Description: "Kubernetes service accounts that are linked to accounts at the cloud provider (IAM). Use these accounts to allow apps to access cloud resources",
		Before:      func(c *cli.Context) error { return ensureClusterSelected() },
		Category:    "Cluster",
		Subcommands: []*cli.Command{
			{
				Name:  "create",
				Usage: "create a new linked service account",
				Action: func(c *cli.Context) error {
					return accountEdit(c.String("account"), false)
				},
				Flags: []cli.Flag{
					accountFlag,
				},
			},
			{
				Name:   "delete",
				Usage:  "deletes an account",
				Action: accountDelete,
				Flags: []cli.Flag{
					accountFlag,
				},
			},
			{
				Name:  "edit",
				Usage: "edit an existing linked service account",
				Action: func(c *cli.Context) error {
					return accountEdit(c.String("account"), true)
				},
				Flags: []cli.Flag{
					accountFlag,
				},
			},
			{
				Name:   "list",
				Usage:  "lists accounts on this cluster",
				Action: accountList,
			},
		},
	},
}

type accountTemplate struct {
	AccountName string
}

func accountEdit(name string, allowOverride bool) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	kclient := ac.kubernetesClient()

	// find existing account with name
	var accountBody []byte
	lsa := &v1alpha1.LinkedServiceAccount{}
	if err = kclient.Get(context.Background(), client.ObjectKey{Name: name}, lsa); err != nil {
		if errors.IsNotFound(err) {
			// read template
			box := utils.DeployResourcesBox()
			f, err := box.Open("templates/linkedaccount.yaml")
			if err != nil {
				return err
			}
			defer f.Close()
			content, err := ioutil.ReadAll(f)
			if err != nil {
				return err
			}

			tmpl, err := template.New("app").Parse(string(content))
			if err != nil {
				return err
			}

			buf := bytes.NewBuffer(nil)
			if err = tmpl.Execute(buf, accountTemplate{name}); err != nil {
				return err
			}
			accountBody = buf.Bytes()
		} else {
			return err
		}
	} else if !allowOverride {
		// in create mode, return already exists error
		return fmt.Errorf("An account named %s already exists", name)
	} else {
		buf := bytes.NewBuffer(nil)
		err = kube.GetKubeEncoder().Encode(lsa, buf)
		if err != nil {
			return err
		}
		accountBody = buf.Bytes()
	}

	// edit
	accountBody, err = utilscli.ExecuteUserEditor(accountBody, fmt.Sprintf("%s.yaml", name))
	if err != nil {
		return err
	}

	obj, _, err := kube.GetKubeDecoder().Decode(accountBody, nil, lsa)
	if err != nil {
		return err
	}

	lsa = obj.(*v1alpha1.LinkedServiceAccount)
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}
	lsa.Spec.Targets = cc.Spec.Targets

	// sync to cluster
	if err = reconcileAccount(ac, lsa); err != nil {
		return err
	}

	fmt.Println("Account updated")
	return nil
}

func accountDelete(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	account := c.String("account")

	kclient := ac.kubernetesClient()
	lsa := &v1alpha1.LinkedServiceAccount{}
	err = kclient.Get(context.Background(), client.ObjectKey{Name: account}, lsa)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// remove
	err = ac.Manager.DeleteLinkedServiceAccount(ac.Cluster, lsa)
	if err != nil {
		return err
	}

	// delete the obj
	err = kclient.Delete(context.Background(), lsa)
	if err != nil {
		return err
	}

	fmt.Println("Successfully deleted account", account)
	return nil
}

func accountList(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}
	kclient := ac.kubernetesClient()

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{
		"Linked Account",
		"Targets Ready",
		"Policies",
	})

	err = resources.ForEach(kclient, &v1alpha1.LinkedServiceAccountList{}, func(item interface{}) error {
		lsa := item.(v1alpha1.LinkedServiceAccount)
		table.Append([]string{
			lsa.Name,
			strings.Join(lsa.Status.LinkedTargets, ","),
			strings.Join(lsa.GetPolicies(), "\n"),
		})
		return nil
	})

	utils.FormatTable(table)
	table.Render()

	if err != nil {
		return err
	}

	return nil
}

func reconcileAccount(ac *activeCluster, account *v1alpha1.LinkedServiceAccount) error {
	needsReconcile, err := account.NeedsReconcile()
	if err != nil {
		return err
	}

	if !needsReconcile {
		return nil
	}

	fmt.Printf("Syncing account %s to %s\n", account.Name, ac.Manager.Cloud())

	kclient := ac.kubernetesClient()

	// create service accounts for each target
	for _, target := range account.Spec.Targets {
		// create account if it doesn't exist in the namespace
		var existing corev1.ServiceAccount
		err = kclient.Get(context.Background(), client.ObjectKey{Namespace: target, Name: account.Name}, &existing)
		if err == nil {
			// already exists
			continue
		} else if !errors.IsNotFound(err) {
			return err
		}

		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: target,
				Name:      account.Name,
			},
		}
		if err = kclient.Create(context.Background(), sa); err != nil {
			return err
		}
	}

	if err = ac.Manager.SyncLinkedServiceAccount(ac.Cluster, account); err != nil {
		return err
	}

	if err = account.UpdateHash(); err != nil {
		return err
	}

	// unfortunately UpdateResource wipes out status updates.. so when status changes are made previously, they'd be gone
	status := account.Status
	_, err = resources.UpdateResource(kclient, account, nil, nil)
	if err != nil {
		return err
	}

	account.Status = status
	return kclient.Status().Update(context.Background(), account)
}

func reconcileAccounts(ac *activeCluster, targets []string) error {
	// reconcile all accounts with new targets (used when cluster targets change)
	kclient := ac.kubernetesClient()
	err := resources.ForEach(kclient, &v1alpha1.LinkedServiceAccountList{}, func(obj interface{}) error {
		lsa := obj.(v1alpha1.LinkedServiceAccount)
		lsa.Spec.Targets = targets
		return reconcileAccount(ac, &lsa)
	})
	if err != nil {
		return err
	}
	return nil
}
