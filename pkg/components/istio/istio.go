package istio

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	istionetworking "istio.io/api/networking/v1beta1"
	istio "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/pkg/utils/cli"
	"github.com/k11n/konstellation/pkg/utils/files"
)

const (
	istioVersion = "1.9.2"
)

func init() {
	components.RegisterComponent(&IstioInstaller{})
}

type IstioInstaller struct {
}

func (i *IstioInstaller) Name() string {
	return "istio"
}

func (i *IstioInstaller) VersionForKube(version string) string {
	return istioVersion
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

	return !strings.HasPrefix(string(output), istioVersion)
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

	parentDir := path.Dir(i.installRoot())
	if err = os.MkdirAll(parentDir, files.DefaultDirectoryMode); err != nil {
		return err
	}

	// clear existing dir
	os.RemoveAll(i.installRoot())
	os.RemoveAll(i.cliPath())

	// run download script
	cmd := exec.Command(installCmd)
	cmd.Dir = parentDir
	cmd.Env = []string{
		fmt.Sprintf("ISTIO_VERSION=%s", istioVersion),
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
		//"--set", "components.citadel.enabled=true",
		//"--set", "components.sidecarInjector.enabled=true",
		"--set", "addonComponents.kiali.enabled=false",
		"--set", "addonComponents.prometheus.enabled=false",
		"--set", "addonComponents.grafana.enabled=false",
		"--set", "values.gateways.istio-ingressgateway.type=NodePort",
		"--set", "values.gateways.enabled=true",
		"--set", "values.global.proxy.holdApplicationUntilProxyStarts=true",
	)
	if err != nil {
		return err
	}

	// 1.6.5 stopped shipping with Gateway, we'll create it ourselves
	gateway := &istio.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: resources.IstioNamespace,
			Name:      resources.GatewayName,
		},
		Spec: istionetworking.Gateway{
			Selector: map[string]string{
				"istio": "ingressgateway",
			},
			Servers: []*istionetworking.Server{
				{
					Port: &istionetworking.Port{
						Name:     "http",
						Number:   80,
						Protocol: "HTTP",
					},
					Hosts: []string{"*"},
				},
			},
		},
	}
	_, err = resources.UpdateResource(kclient, gateway, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

func (i *IstioInstaller) cliPath() string {
	return path.Join(cli.GetBinDir(), "istioctl")
}

func (i *IstioInstaller) installRoot() string {
	return path.Join(cli.GetRootDir(), fmt.Sprintf("istio-%s", istioVersion))
}
