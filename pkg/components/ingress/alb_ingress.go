package ingress

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/pkg/utils/cli"
)

const (
	albIngressName    = "alb-ingress-controller"
	albRoleAnnotation = "eks.amazonaws.com/role-arn"
)

func init() {
	components.RegisterComponent(&AWSALBIngress{})
}

type AWSALBIngress struct {
}

func (i *AWSALBIngress) Name() string {
	return "ingress.awsalb"
}

func (i *AWSALBIngress) Version() string {
	return "1.1.6"
}

func (i *AWSALBIngress) InstallComponent(kclient client.Client) error {
	// deploy roles xml
	url := fmt.Sprintf("https://raw.githubusercontent.com/kubernetes-sigs/aws-alb-ingress-controller/v%s/docs/examples/rbac-role.yaml", i.Version())
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
		"kubernetes.io/ingress.class":            "alb",
		"alb.ingress.kubernetes.io/scheme":       "internet-facing",
		"alb.ingress.kubernetes.io/listen-ports": listeners,
	}
	return
}

func (i *AWSALBIngress) deploymentForIngress(cc *v1alpha1.ClusterConfig) *appsv1.Deployment {
	labels := map[string]string{
		"app.kubernetes.io/name": albIngressName,
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
							Image: "docker.io/amazon/aws-alb-ingress-controller:v1.1.6",
						},
					},
					ServiceAccountName: albIngressName,
				},
			},
		},
	}
	return dep
}
