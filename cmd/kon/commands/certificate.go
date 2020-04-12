package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/cloud/types"
	"github.com/davidzhao/konstellation/pkg/resources"
	"github.com/davidzhao/konstellation/pkg/utils/files"
)

var CertificateCommands = []*cli.Command{
	{
		Name:  "certificate",
		Usage: "Add, remove, and manage SSL certificates",
		Action: func(c *cli.Context) error {
			return cli.ShowAppHelp(c)
		},
		Subcommands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "List certificates in Kube",
				Action: certList,
			},
			{
				Name:   "sync",
				Usage:  "Sync certificates from cloud provider",
				Action: certSync,
			},
			{
				Name:   "import",
				Usage:  "Imports an existing certificate to provider's certificate management",
				Action: certImport,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "domain",
						Usage:    "the domain to load this certificate",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "certificate",
						Usage:    "certificate file",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "private-key",
						Usage:    "private key file",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "chain",
						Usage: "intermediate certificate chain file",
					},
				},
			},
		},
	},
}

func certList(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	kclient := ac.kubernetesClient()
	certs, err := resources.ListCertificates(kclient)
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{
		"ID", "Domain", "Issuer", "Expiration", "Status",
	})

	for _, c := range certs {
		cs := &c.Spec
		table.Append([]string{
			c.Name,
			cs.Domain,
			cs.Issuer,
			cs.ExpiresAt.Format("2006-01-02"),
			cs.Status,
		})
	}
	utils.FormatTable(table)
	table.Render()

	return nil
}

func certSync(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	certs, err := ac.Manager.CertificateProvider().ListCertificates(context.TODO())
	if err != nil {
		return err
	}

	kclient := ac.kubernetesClient()
	existingCerts, err := resources.ListCertificates(kclient)
	if err != nil {
		return err
	}

	seenCerts := map[string]bool{}
	for _, cert := range certs {
		seenCerts[cert.ID] = true
		if err := syncCertificate(kclient, cert); err != nil {
			return err
		}
	}

	for _, existingCert := range existingCerts {
		if seenCerts[existingCert.Name] {
			continue
		}
		if err := kclient.Delete(context.TODO(), &existingCert); err != nil {
			return err
		}
	}
	fmt.Printf("Successfully synced %d certificates\n", len(certs))
	return nil
}

func certImport(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}
	domain := c.String("domain")
	certPath := c.String("certificate")
	pkeyPath := c.String("private-key")
	chainPath := c.String("chain")

	cert, err := files.ReadFile(certPath)
	if err != nil {
		return err
	}

	pkey, err := files.ReadFile(pkeyPath)
	if err != nil {
		return err
	}

	var chain []byte
	if chainPath != "" {
		chain, err = files.ReadFile(chainPath)
		if err != nil {
			return err
		}
	}

	// find existing cert if exists
	existingID := ""
	if existingCert, err := resources.GetCertificateForDomain(ac.kubernetesClient(), domain); err == nil {
		existingID = existingCert.Spec.ProviderID
	}

	certificate, err := ac.Manager.CertificateProvider().ImportCertificate(context.TODO(),
		cert, pkey, chain, existingID)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully imported certificate %s\n", certificate.Domain)

	// import it into cloud
	err = syncCertificate(ac.kubernetesClient(), certificate)
	if err != nil {
		return errors.Wrap(err, "Could not sync imported cert, please sync again later")
	}

	return nil
}

func syncCertificate(kclient client.Client, cert *types.Certificate) error {
	existing := v1alpha1.CertificateRef{
		ObjectMeta: metav1.ObjectMeta{
			Name: cert.ID,
		},
	}

	_, err := controllerutil.CreateOrUpdate(context.TODO(), kclient, &existing, func() error {
		existing.Labels = map[string]string{
			resources.DOMAIN_LABEL: cert.Domain,
		}
		existing.Spec = v1alpha1.CertificateRefSpec{
			ProviderID:         cert.ProviderID,
			Domain:             cert.Domain,
			Issuer:             cert.Issuer,
			Status:             cert.Status.String(),
			KeyAlgorithm:       cert.KeyAlgorithm,
			SignatureAlgorithm: cert.SignatureAlgorithm,
		}
		if cert.ExpiresAt != nil {
			existing.Spec.ExpiresAt = metav1.NewTime(*cert.ExpiresAt)
		}
		return nil
	})
	return err
}
