package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/davidzhao/konstellation/pkg/components/kubedash"
	"github.com/pkg/browser"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

var WebCommands = []*cli.Command{
	&cli.Command{
		Name:   "dashboard",
		Usage:  "Launch Kubernetes dashboard",
		Action: webDashboard,
	},
}

func webDashboard(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	// print token
	token, err := ac.Cloud.KubernetesProvider().GetAuthToken(context.TODO(), ac.Cluster)
	if err != nil {
		return errors.Wrap(err, "failed to get authentication token")
	}

	fmt.Printf("Authentication token (copy and paste to login)\n%s\n", token.Status.Token)

	// run proxy
	cmd := exec.Command("kubectl", "proxy")
	err = cmd.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start kubernetes proxy")
	}

	// handle signal for clean shutdown
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	// launch web browser after delay
	time.Sleep(3 * time.Second)
	browser.OpenURL(kubedash.LOCAL_URL)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		cmd.Process.Signal(syscall.SIGTERM)
		done <- true
	}()

	<-done
	return nil
}
