package ingress

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cast"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/pkg/utils/cli"
)

const (
	albIngressName       = "alb-ingress-controller"
	albRoleAnnotation    = "eks.amazonaws.com/role-arn"
	albControllerVersion = "1.1.8"
)

var alblog = logf.Log.WithName("component.ALBIngress")

func init() {
	components.RegisterComponent(&AWSALBIngress{})
}

type AWSALBIngress struct {
}

func (i *AWSALBIngress) Name() string {
	return "ingress.awsalb"
}

func (i *AWSALBIngress) VersionForKube(version string) string {
	return albControllerVersion
}

func (i *AWSALBIngress) InstallComponent(kclient client.Client) error {
	// deploy roles yaml
	url := fmt.Sprintf("https://raw.githubusercontent.com/kubernetes-sigs/aws-alb-ingress-controller/v%s/docs/examples/rbac-role.yaml", albControllerVersion)
	err := cli.KubeApply(url)
	if err != nil {
		return nil
	}

	// get cluster config and alb service account to annotate
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}

	svcAccount := &corev1.ServiceAccount{}
	err = kclient.Get(context.TODO(), types.NamespacedName{
		Name:      albIngressName,
		Namespace: resources.KubeSystemNamespace,
	}, svcAccount)
	if err != nil {
		return err
	}

	svcAccount.Annotations[albRoleAnnotation] = cc.Spec.AWS.AlbRoleArn
	err = kclient.Update(context.TODO(), svcAccount)
	if err != nil {
		return err
	}

	// last step to create deployment
	dep := i.deploymentForIngress(cc)

	_, err = resources.UpdateResource(kclient, dep, nil, nil)

	return err
}

func (i *AWSALBIngress) GetIngressAnnotations(kclient client.Client, tlsHosts []string) (annotations map[string]string, err error) {
	// https://kubernetes-sigs.github.io/aws-alb-ingress-controller/guide/ingress/annotation/
	// ingress could perform autodiscovery
	listeners := `[{"HTTP": 80}]`
	if len(tlsHosts) > 0 {
		listeners = `[{"HTTP": 80}, {"HTTPS": 443}]`
	}

	annotations = map[string]string{
		"kubernetes.io/ingress.class":               "alb",
		"alb.ingress.kubernetes.io/ip-address-type": "dualstack",
		"alb.ingress.kubernetes.io/listen-ports":    listeners,
		"alb.ingress.kubernetes.io/scheme":          "internet-facing",
	}

	//

	// find istio status port
	svc, err := resources.GetService(kclient, resources.IstioNamespace, resources.IngressBackendName)
	if err != nil {
		return
	}

	for _, p := range svc.Spec.Ports {
		if p.Name == "status-port" {
			annotations["alb.ingress.kubernetes.io/healthcheck-port"] = cast.ToString(p.NodePort)
			annotations["alb.ingress.kubernetes.io/healthcheck-path"] = resources.IngressHealthPath
		}
	}

	// get all certs and match against
	certs, err := resources.ListCertificates(kclient)
	if err != nil {
		return
	}

	arns := make([]string, 0)
	seenCerts := make(map[string]bool, 0)
	for _, host := range tlsHosts {
		var matchingCert *v1alpha1.CertificateRef
		for i := range certs {
			cert := &certs[i]
			if resources.CertificateCovers(cert.Spec.Domain, host) {
				if matchingCert == nil || cert.Spec.ExpiresAt.After(matchingCert.Spec.ExpiresAt.Time) {
					matchingCert = cert
				}
			}
		}
		if matchingCert != nil {
			//alblog.Info("certificate matching host", "certDomain", matchingCert.Spec.Domain,
			//	"certID", matchingCert.Name, "hostDomain", host)
			arn := matchingCert.Spec.ProviderID
			if seenCerts[arn] {
				// certificate already included
				continue
			}
			seenCerts[arn] = true
			arns = append(arns, arn)
		}
	}

	if len(arns) > 0 {
		annotations["alb.ingress.kubernetes.io/certificate-arn"] = strings.Join(arns, ",")
	}
	return
}

func (i *AWSALBIngress) deploymentForIngress(cc *v1alpha1.ClusterConfig) *appsv1.Deployment {
	labels := map[string]string{
		resources.KubeAppLabel: albIngressName,
	}
	// mapped from: https://raw.githubusercontent.com/kubernetes-sigs/aws-alb-ingress-controller/v1.1.6/docs/examples/alb-ingress-controller.yaml
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      albIngressName,
			Namespace: resources.KubeSystemNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: albIngressName,
							Args: []string{
								"--ingress-class=alb",
								fmt.Sprintf("--cluster-name=%s", cc.Name),
							},
							Image: "docker.io/amazon/aws-alb-ingress-controller:v" + albControllerVersion,
						},
					},
					ServiceAccountName: albIngressName,
				},
			},
		},
	}
	return dep
}
