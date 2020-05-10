package istio

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/lytics/base62"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/pkg/utils/cli"
	"github.com/k11n/konstellation/pkg/utils/files"
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
		"--set", "addonComponents.grafana.enabled=true",
		"--set", "values.kiali.dashboard.grafanaURL=http://grafana:3000",
		"--set", "values.gateways.istio-ingressgateway.type=NodePort",
		"--set", "values.gateways.enabled=true",
	)
	if err != nil {
		return err
	}

	// create a secret for kiali
	data, _ := uuid.New().MarshalBinary()
	base62.StdEncoding.EncodeToString(data)
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: resources.IstioNamespace,
			Name:      "kiali",
			Labels: map[string]string{
				"app": "kiali",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username":   []byte("kiali"),
			"passphrase": []byte("kiali"),
		},
	}

	existing := corev1.Secret{}
	key, err := client.ObjectKeyFromObject(&secret)
	err = kclient.Get(context.TODO(), key, &existing)
	if err != nil {
		if errors.IsNotFound(err) {
			err = kclient.Create(context.TODO(), &secret)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	err = kclient.Update(context.TODO(), &secret)
	return err
}

func (i *IstioInstaller) cliPath() string {
	return path.Join(cli.GetBinDir(), "istioctl")
}

func (i *IstioInstaller) installRoot() string {
	return path.Join(cli.GetRootDir(), fmt.Sprintf("istio-%s", i.Version()))
}
