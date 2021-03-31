package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cast"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/k11n/konstellation/api/v1alpha1"
	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/pkg/utils/cli"
)

const (
	albIngressName        = "alb-ingress-controller"
	albServiceAccountName = "aws-load-balancer-controller"
	albRoleAnnotation     = "eks.amazonaws.com/role-arn"
	albControllerVersion  = "2.1.2"
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
	// deploy cert manager
	url := fmt.Sprintf("https://github.com/jetstack/cert-manager/releases/download/v1.0.2/cert-manager.yaml")
	err := cli.KubeApply(url)
	if err != nil {
		return nil
	}

	// edit cluster name
	templateUrl := "https://raw.githubusercontent.com/kubernetes-sigs/aws-load-balancer-controller/v2.1.2/docs/install/v2_1_2_full.yaml"
	resp, err := http.Get(templateUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// replace with cluster name
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}
	content := strings.ReplaceAll(string(data), "--cluster-name=your-cluster-name", fmt.Sprintf("--cluster-name=%s", cc.Name))
	reader := bytes.NewReader([]byte(content))
	if err = cli.KubeApplyReader(reader); err != nil {
		return err
	}

	// alb service account to annotate
	svcAccount := &corev1.ServiceAccount{}
	err = kclient.Get(context.TODO(), types.NamespacedName{
		Name:      albServiceAccountName,
		Namespace: resources.KubeSystemNamespace,
	}, svcAccount)
	if err != nil {
		return err
	}

	svcAccount.Annotations[albRoleAnnotation] = cc.Status.AWS.AlbRoleArn
	err = kclient.Update(context.TODO(), svcAccount)
	if err != nil {
		return err
	}

	return err
}

func (i *AWSALBIngress) ConfigureIngress(kclient client.Client, ingress *netv1.Ingress, irs []*v1alpha1.IngressRequest) error {
	cc, err := resources.GetClusterConfig(kclient)
	if err != nil {
		return err
	}

	// if grpc mode, operate differently
	isGrpc := false
	for _, ir := range irs {
		if ir.Spec.AppProtocol == "grpc" {
			isGrpc = true
		}
	}

	annotations := map[string]string{
		"kubernetes.io/ingress.class":                        "alb",
		"alb.ingress.kubernetes.io/scheme":                   "internet-facing",
		"alb.ingress.kubernetes.io/load-balancer-attributes": "routing.http2.enabled=true",
	}

	// attach dualstack LB if we have IPV6 enabled on the subnet
	if cc.Spec.EnableIpv6 && cc.Status.AWS.Ipv6Cidr != "" {
		annotations["alb.ingress.kubernetes.io/ip-address-type"] = "dualstack"
	}

	// find istio status port
	svc, err := resources.GetService(kclient, resources.IstioNamespace, resources.IngressBackendName)
	if err != nil {
		return err
	}

	if isGrpc {
		annotations["alb.ingress.kubernetes.io/backend-protocol-version"] = "GRPC"
	} else {
		// point to HTTP port for status, otherwise use default
		for _, p := range svc.Spec.Ports {
			if p.Name == "status-port" {
				annotations["alb.ingress.kubernetes.io/healthcheck-port"] = cast.ToString(p.NodePort)
				annotations["alb.ingress.kubernetes.io/healthcheck-path"] = resources.IngressHealthPath
			}
		}
	}

	listeners := []map[string]int{}
	if !isGrpc {
		listeners = append(listeners, map[string]int{
			"HTTP": 80,
		})
	}

	requiresHttps := false
	var customAnnotations map[string]string

	// unique list of hosts
	hostsSeen := make(map[string]bool)
	var hosts []string
	for _, ir := range irs {
		for _, host := range ir.Spec.Hosts {
			if !hostsSeen[host] {
				hostsSeen[host] = true
				hosts = append(hosts, host)
			}
		}
		if ir.Spec.RequireHTTPS {
			requiresHttps = true
		}
		// use the first annotations that we could find
		if len(ir.Spec.Annotations) != 0 && customAnnotations != nil {
			customAnnotations = ir.Spec.Annotations
		}
	}

	// get all certs and match against
	certMap, err := resources.GetCertificatesForHosts(kclient, hosts)
	if err != nil {
		return err
	}

	var tlsHosts []string
	for _, host := range hosts {
		// grpc requires https, so we'll always enable
		if certMap[host] != nil || isGrpc {
			tlsHosts = append(tlsHosts, host)
		}
	}

	// configure TLSHosts field
	if len(tlsHosts) != 0 {
		ingress.Spec.TLS = []netv1.IngressTLS{
			{
				Hosts: tlsHosts,
			},
		}
	}

	arns := make([]string, 0)
	seenCerts := make(map[string]bool, 0)
	for _, host := range hosts {
		cert := certMap[host]
		if cert == nil {
			continue
		}

		//alblog.Info("certificate matching host", "certDomain", matchingCert.Spec.Domain,
		//	"certID", matchingCert.Name, "hostDomain", host)
		arn := cert.Spec.ProviderID
		if seenCerts[arn] {
			// certificate already included
			continue
		}
		seenCerts[arn] = true
		arns = append(arns, arn)
	}

	if len(arns) > 0 {
		annotations["alb.ingress.kubernetes.io/certificate-arn"] = strings.Join(arns, ",")
		listeners = append(listeners, map[string]int{
			"HTTPS": 443,
		})
		// see if we need HTTPS redirection
		if requiresHttps && !isGrpc {
			pathType := netv1.PathTypePrefix
			annotations["alb.ingress.kubernetes.io/actions.ssl-redirect"] = `{"Type": "redirect", "RedirectConfig": { "Protocol": "HTTPS", "Port": "443", "StatusCode": "HTTP_301"}}`
			paths := []netv1.HTTPIngressPath{
				{
					Path:     "/",
					PathType: &pathType,
					Backend: netv1.IngressBackend{
						Service: &netv1.IngressServiceBackend{
							Name: "ssl-redirect",
							Port: netv1.ServiceBackendPort{
								Name: "use-annotation",
							},
						},
					},
				},
			}
			rule := &ingress.Spec.Rules[0]
			paths = append(paths, rule.IngressRuleValue.HTTP.Paths...)
			rule.IngressRuleValue.HTTP.Paths = paths
		}
	}

	// allow config overrides
	for key, val := range customAnnotations {
		if strings.HasPrefix(key, "alb.ingress") {
			annotations[key] = val
		}
	}

	listenersJson, err := json.Marshal(listeners)
	if err != nil {
		return err
	}
	annotations["alb.ingress.kubernetes.io/listen-ports"] = string(listenersJson)

	if ingress.Annotations == nil {
		ingress.Annotations = annotations
	} else {
		for key, val := range annotations {
			ingress.Annotations[key] = val
		}
	}

	return nil
}
