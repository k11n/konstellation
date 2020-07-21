package cli

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cast"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k11n/konstellation/pkg/resources"
	"github.com/k11n/konstellation/pkg/utils/assets"
)

var KubeDisplayOutput = false

func KubeCtl(args ...string) error {
	cmd := exec.Command("kubectl", args...)
	if KubeDisplayOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func KubeApplyReader(reader io.Reader) error {
	args := []string{
		"apply", "-f", "-",
	}
	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = reader
	if KubeDisplayOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func KubeApplyFromBox(filename string, context string) error {
	filepath, err := assets.TempfileFromDeployResource(filename)
	if err != nil {
		return err
	}
	defer os.Remove(filepath)

	args := []string{
		"apply", "-f", filepath,
	}
	if context != "" {
		args = append(args, "--context", context)
	}
	return KubeCtl(args...)
}

func KubeApply(url string) error {
	args := []string{
		"apply", "-f", url,
	}
	cmd := exec.Command("kubectl", args...)
	if KubeDisplayOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

type KubeProxy struct {
	Namespace   string
	Service     string
	ServicePort int32
	LocalPort   int32
	command     *exec.Cmd
	started     bool
	sigChan     chan os.Signal
	doneChan    chan bool
}

var (
	usedPorts = sync.Map{}
)

func NewKubeProxy() *KubeProxy {
	return &KubeProxy{}
}

func NewKubeProxyForService(kclient client.Client, namespace, service string, port interface{}) (kp *KubeProxy, err error) {
	// find port for service
	svc, err := resources.GetService(kclient, namespace, service)
	if err != nil {
		return
	}

	if len(svc.Spec.Ports) == 0 {
		err = fmt.Errorf("Service does not have any ports")
		return
	}

	portNumber := cast.ToInt32(port)
	if portNumber == 0 {
		// not a proper port number
		if port == nil || port == "" {
			// no indicated port, use the first
			portNumber = svc.Spec.Ports[0].Port
		} else if portName, ok := port.(string); ok {
			// port name, search in service
			for _, p := range svc.Spec.Ports {
				if p.Name == portName {
					portNumber = p.Port
					break
				}
			}
			// no port name found
			if portNumber == 0 {
				err = fmt.Errorf("service %s doesn't have a port named %s", service, portName)
				return
			}
		} else {
			err = fmt.Errorf("incorrect port %v (%T)", port, port)
		}
	}

	kp = &KubeProxy{
		Namespace:   namespace,
		Service:     service,
		ServicePort: portNumber,
	}

	return
}

func (p *KubeProxy) Start() error {
	if p.started {
		return fmt.Errorf("Proxy is already started")
	}

	defPort := int32(7001)
	if p.Service != "" {
		defPort = p.ServicePort
	}

	// find unused port
	port, err := p.findUnusedPort(defPort)
	if err != nil {
		return err
	}

	// launches port-forward if a service is defined
	var args []string
	if p.Service != "" {
		args = []string{
			"port-forward", "-n", p.Namespace,
			fmt.Sprintf("service/%s", p.Service),
			fmt.Sprintf("%d:%d", port, p.ServicePort),
		}
	} else {
		args = []string{
			"proxy", "-p", cast.ToString(port),
		}
	}

	usedPorts.Store(port, true)
	//fmt.Printf("Running kubectl %s\n", strings.Join(args, " "))
	p.command = exec.Command("kubectl", args...)
	err = p.command.Start()
	if err != nil {
		return err
	}
	p.LocalPort = port
	p.started = true

	return nil
}

func (p *KubeProxy) WaitUntilCanceled() {
	if !p.started || p.sigChan != nil {
		return
	}
	// handle signal for clean shutdown
	p.sigChan = make(chan os.Signal, 1)
	p.doneChan = make(chan bool, 1)

	signal.Notify(p.sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-p.sigChan
		p.Stop()
	}()

	<-p.doneChan
}

func (p *KubeProxy) Stop() {
	if !p.started {
		return
	}
	if p.command != nil {
		p.command.Process.Signal(syscall.SIGTERM)
	}
	if p.doneChan != nil {
		p.doneChan <- true
		close(p.doneChan)
		p.doneChan = nil
	}
	if p.sigChan != nil {
		signal.Stop(p.sigChan)
		close(p.sigChan)
		p.sigChan = nil
	}
	usedPorts.Delete(p.LocalPort)
	p.started = false
}

func (p *KubeProxy) URL() string {
	return fmt.Sprintf("http://%s", p.HostWithPort())
}

func (p *KubeProxy) HostWithPort() string {
	return fmt.Sprintf("localhost:%d", p.LocalPort)
}

func (p *KubeProxy) findUnusedPort(initial int32) (int32, error) {
	for port := initial; port < initial+1000; port += 1 {
		if _, ok := usedPorts.Load(port); ok {
			// used by another process
			continue
		}
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%d", port))
		if err != nil {
			return 0, err
		}

		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			continue
		}
		defer l.Close()
		return int32(l.Addr().(*net.TCPAddr).Port), nil
	}
	return 0, fmt.Errorf("could not find unused port")
}
