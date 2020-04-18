package ingress

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/davidzhao/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/davidzhao/konstellation/pkg/components"
	"github.com/davidzhao/konstellation/pkg/utils/cli"
)

const (
	albIngressName = "alb-ingress-controller"
)

func init() {
	components.RegisterComponent(&AWSALBIngress{})
}

type AWSALBIngress struct {
	Cluster string
}

func (i *AWSALBIngress) Name() string {
	return "ingress.awsalb"
}

func (i *AWSALBIngress) Version() string {
	return "1.1.6"
}

func (i *AWSALBIngress) NeedsCLI() bool {
	return false
}

func (i *AWSALBIngress) InstallCLI() error {
	return nil
}

func (i *AWSALBIngress) InstallComponent(kclient client.Client) error {
	// deploy roles xml
	url := fmt.Sprintf("https://raw.githubusercontent.com/kubernetes-sigs/aws-alb-ingress-controller/v%s/docs/examples/rbac-role.yaml", i.Version())
	err := cli.KubeApply(url)
	if err != nil {
		return nil
	}
	dep := i.deploymentForIngress()
	existing := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      albIngressName,
			Namespace: "kube-system",
		},
	}
	_, err = controllerutil.CreateOrUpdate(context.TODO(), kclient, existing, func() error {
		if existing.CreationTimestamp.IsZero() {
			existing.Spec = dep.Spec
		}
		return nil
	})
	return err
}

func (i *AWSALBIngress) GetIngressAnnotations(kclient client.Client, requests []v1alpha1.IngressRequest) (annotations map[string]string, err error) {
	// https://kubernetes-sigs.github.io/aws-alb-ingress-controller/guide/ingress/annotation/
	// ingress could perform autodiscovery
	//cc, err := resources.GetClusterConfig(kclient)
	//if err != nil {
	//	return
	//}
	//
	//aws := cc.Spec.AWS
	//subnetIds := make([]string, 0, len(aws.PublicSubnets))
	//for _, subnet := range aws.PublicSubnets {
	//	subnetIds = append(subnetIds, subnet.SubnetId)
	//}
	annotations = map[string]string{
		"kubernetes.io/ingress.class":      "alb",
		"alb.ingress.kubernetes.io/scheme": "internet-facing",
	}
	return
}

func (i *AWSALBIngress) deploymentForIngress() *appsv1.Deployment {
	labels := map[string]string{
		"app.kubernetes.io/name": albIngressName,
	}
	// mapped from: https://raw.githubusercontent.com/kubernetes-sigs/aws-alb-ingress-controller/v1.1.6/docs/examples/alb-ingress-controller.yaml
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      albIngressName,
			Namespace: "kube-system",
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
								fmt.Sprintf("--cluster-name=%s", i.Cluster),
							},
							//Env: []corev1.EnvVar{
							//	{
							//		Name: "AWS_ACCESS_KEY_ID",
							//		Value: "",
							//	},
							//	{
							//		Name: "AWS_SECRET_ACCESS_KEY",
							//		Value: "",
							//	},
							//},
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
