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
)

func RunBufferedCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

func RunCommandWithStd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

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
	command  *exec.Cmd
	started  bool
	Port     int
	sigChan  chan os.Signal
	doneChan chan bool
}

func NewKubeProxy() *KubeProxy {
	return &KubeProxy{}
}

func (p *KubeProxy) Start() error {
	if p.started {
		return fmt.Errorf("Proxy is already started")
	}

	// find unused port
	port, err := p.findUnusedPort(8001)
	if err != nil {
		return err
	}

	p.command = exec.Command(
		"kubectl", "proxy", "-p", strconv.Itoa(port),
	)
	err = p.command.Start()
	if err != nil {
		return err
	}
	p.Port = port
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
	return fmt.Sprintf("http://localhost:%d", p.Port)
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
