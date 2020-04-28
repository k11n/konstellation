package cli

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/davidzhao/konstellation/pkg/resources"
)

func KubeCtl(args ...string) error {
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func KubeApplyReader(reader io.Reader) error {
	args := []string{
		"apply", "-f", "-",
	}
	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = reader
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func KubeApply(url string) error {
	args := []string{
		"apply", "-f", url,
	}
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type KubeProxy struct {
	Namespace   string
	Service     string
	ServicePort int
	LocalPort   int
	command     *exec.Cmd
	started     bool
	sigChan     chan os.Signal
	doneChan    chan bool
}

func NewKubeProxy() *KubeProxy {
	return &KubeProxy{}
}

func NewKubeProxyForService(kclient client.Client, namespace, service string) (kp *KubeProxy, err error) {
	// find port for service
	svc, err := resources.GetService(kclient, namespace, service)
	if err != nil {
		return
	}

	if len(svc.Spec.Ports) == 0 {
		err = fmt.Errorf("Service does not have any ports")
		return
	}
	kp = &KubeProxy{
		Namespace:   namespace,
		Service:     service,
		ServicePort: int(svc.Spec.Ports[0].Port),
	}

	return
}

func (p *KubeProxy) Start() error {
	if p.started {
		return fmt.Errorf("Proxy is already started")
	}

	// find unused port
	port, err := p.findUnusedPort(7001)
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
			"proxy", "-p", strconv.Itoa(port),
		}
	}

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

func (p *KubeProxy) WaitUntilDone() {
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
		p.sigChan = nil
	}
	p.started = false
}

func (p *KubeProxy) HostWithPort() string {
	return fmt.Sprintf("http://localhost:%d", p.LocalPort)
}

func (p *KubeProxy) findUnusedPort(initial int) (int, error) {
	for port := initial; port < initial+1000; port += 1 {
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%d", port))
		if err != nil {
			return 0, err
		}

		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			continue
		}
		defer l.Close()
		return l.Addr().(*net.TCPAddr).Port, nil
	}
	return 0, fmt.Errorf("could not find unused port")
}
