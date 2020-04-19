package istio

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/davidzhao/konstellation/cmd/kon/utils"
	"github.com/davidzhao/konstellation/pkg/components"
	"github.com/davidzhao/konstellation/pkg/utils/cli"
	"github.com/davidzhao/konstellation/pkg/utils/files"
)

const (
	istioNamespace   = "istio-system"
	istioIngressName = "istio-ingressgateway"
)

func init() {
	components.RegisterComponent(&IstioInstaller{})
}

type IstioInstaller struct {
}

func (i *IstioInstaller) Name() string {
	return "istio"
}

func (i *IstioInstaller) Version() string {
	return "1.5.1"
}

// returns true if CLI is needed and has not yet been installed
func (i *IstioInstaller) NeedsCLI() bool {
	p := i.cliPath()
	if _, err := os.Stat(p); err != nil {
		return true
	}

	// check version
	output, err := cli.RunBufferedCommand(i.cliPath(), "--remote=false", "version")
	if err != nil {
		return true
	}

	return !strings.HasPrefix(string(output), i.Version())
}

// installs CLI locally
func (i *IstioInstaller) InstallCLI() error {
	tmpdir, err := ioutil.TempDir("", "istio")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	// download install file
	installCmd := path.Join(tmpdir, "downloadIstio")
	err = files.DownloadFile(installCmd, "https://istio.io/downloadIstio")
	if err != nil {
		return err
	}
	if err = os.Chmod(installCmd, files.ExecutableFileMode); err != nil {
		return err
	}

	// run download script
	cmd := exec.Command(installCmd)
	cmd.Dir = path.Dir(i.installRoot())
	cmd.Env = []string{
		fmt.Sprintf("ISTIO_VERSION=%s", i.Version()),
	}
	if err = cmd.Run(); err != nil {
		return err
	}

	// should be done
	if err = os.MkdirAll(cli.GetBinDir(), files.DefaultDirectoryMode); err != nil {
		return err
	}

	// now symlink
	return os.Symlink(path.Join(i.installRoot(), "bin", "istioctl"), i.cliPath())
}

// installs the component onto the kube cluster
func (i *IstioInstaller) InstallComponent(kclient client.Client) error {
	err := cli.RunCommandWithStd(i.cliPath(), "manifest", "apply",
		"--skip-confirmation",
		"--set", "components.citadel.enabled=true", // citadel is required by the sidecar injector
		"--set", "components.sidecarInjector.enabled=true",
		"--set", "addonComponents.kiali.enabled=true",
		"--set", "addonComponents.grafana.enabled=true")
	if err != nil {
		return err
	}

	// Delete the default LB service it opens
	ingressKey := client.ObjectKey{
		Namespace: istioNamespace,
		Name:      istioIngressName,
	}
	ingressSvc := &corev1.Service{}
	err = utils.WaitUntilComplete(utils.ShortTimeoutSec, utils.MediumCheckInterval, func() (bool, error) {
		err := kclient.Get(context.TODO(), ingressKey, ingressSvc)
		if err == nil {
			return true, nil
		}
		err = client.IgnoreNotFound(err)
		if err != nil {
			return false, err
		} else {
			return false, nil
		}
	})
	if err != nil {
		return err
	}
	// ignore delete errors
	kclient.Delete(context.TODO(), ingressSvc)

	// create a new service
	ingressSvc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: istioNamespace,
			Name:      istioIngressName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app":   istioIngressName,
				"istio": "ingressgateway",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(80),
					Port:       80,
				},
			},
		},
	}

	return kclient.Create(context.TODO(), ingressSvc)
}

func (i *IstioInstaller) cliPath() string {
	return path.Join(cli.GetBinDir(), "istioctl")
}

func (i *IstioInstaller) installRoot() string {
	return path.Join(cli.GetRootDir(), fmt.Sprintf("istio-%s", i.Version()))
}
