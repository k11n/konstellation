package commands

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/manifoldco/promptui"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cast"
	"github.com/thoas/go-funk"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/k11n/konstellation/cmd/kon/kube"
	"github.com/k11n/konstellation/cmd/kon/utils"
	"github.com/k11n/konstellation/pkg/apis/k11n/v1alpha1"
	"github.com/k11n/konstellation/pkg/resources"
	utilscli "github.com/k11n/konstellation/pkg/utils/cli"
)

var (
	targetFlag = &cli.StringFlag{
		Name:    "target",
		Aliases: []string{"t"},
		Usage:   "a specific target",
	}
	releaseFlag = &cli.StringFlag{
		Name:    "release",
		Aliases: []string{"r"},
		Usage:   "release of the app, defaults to the active release",
	}
	podFlag = &cli.StringFlag{
		Name:    "pod",
		Aliases: []string{"p"},
		Usage:   "a specific pod to use",
	}
)

var AppCommands = []*cli.Command{
	{
		Name:  "app",
		Usage: "App management",
		Before: func(c *cli.Context) error {
			return ensureClusterSelected()
		},
		Category: "App",
		Subcommands: []*cli.Command{
			{
				Name:      "deploy",
				Usage:     "Deploy a new version of an app",
				Action:    appDeploy,
				ArgsUsage: "<app>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "tag",
						Usage:    "image tag to use",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "app",
						Usage:    "app to deploy",
						Required: true,
					},
				},
			},
			{
				Name:      "edit",
				Usage:     "Edit an app's configuration",
				Action:    appEdit,
				ArgsUsage: "<app>",
			},
			{
				Name:   "list",
				Usage:  "List apps on this cluster",
				Action: appList,
				Flags: []cli.Flag{
					targetFlag,
				},
			},
			{
				Name:      "load",
				Usage:     "Load app into Kubernetes (same as kube apply -f)",
				ArgsUsage: "<app.yaml>",
				Action:    appLoad,
			},
			{
				Name:      "local",
				Usage:     "Run app locally with config environment",
				ArgsUsage: "<appName> <executable> [args...]",
				Action:    appLocal,
				Flags: []cli.Flag{
					targetFlag,
				},
			},
			{
				Name:      "logs",
				Usage:     "Print logs from a pod for the app",
				ArgsUsage: "<app>",
				Action:    appLogs,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "follow",
						Aliases: []string{"f"},
						Usage:   "follow logs",
					},
					podFlag,
					targetFlag,
					releaseFlag,
				},
			},
			{
				Name:   "new",
				Usage:  "Create a new app.yaml from template",
				Action: appNew,
			},
			{
				Name:      "pods",
				Usage:     "List pods for this app",
				ArgsUsage: "<app>",
				Action:    appPods,
				Flags: []cli.Flag{
					targetFlag,
					releaseFlag,
				},
			},
			{
				Name:      "rollback",
				Usage:     "Rolls back a bad release and deploys a previous one",
				ArgsUsage: "<app>",
				Action:    appRollback,
				Flags: []cli.Flag{
					targetFlag,
					releaseFlag,
				},
			},
			{
				Name:      "shell",
				Usage:     "Get shell access into a pod with the app",
				ArgsUsage: "<app>",
				Action:    appShell,
				Flags: []cli.Flag{
					podFlag,
					targetFlag,
					releaseFlag,
					&cli.StringFlag{
						Name:    "shell",
						Aliases: []string{"s"},
						Usage:   "command to run as the shell",
						Value:   "/bin/sh",
					},
				},
			},
			{
				Name:      "status",
				Usage:     "Information about the app and its targets",
				Action:    appStatus,
				ArgsUsage: "<app>",
				Flags: []cli.Flag{
					targetFlag,
				},
			},
		},
	},
}

func appList(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	requiredTarget := c.String("target")
	fmt.Printf("Listing apps on %s\n", ac.Cluster)
	apps, err := resources.ListApps(ac.kubernetesClient())
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"App", "Targets", "Ports"})
	for _, app := range apps {
		targets := []string{}
		for _, target := range app.Spec.Targets {
			targets = append(targets, target.Name)
		}
		if requiredTarget != "" && !funk.Contains(targets, requiredTarget) {
			continue
		}

		var portsStr []string

		for _, port := range app.Spec.Ports {
			portsStr = append(portsStr, fmt.Sprintf("%s-%d", port.Name, port.Port))
		}
		table.Append([]string{
			app.Name,
			strings.Join(targets, ", "),
			strings.Join(portsStr, ", "),
		})
	}
	utils.FormatTable(table)
	table.Render()
	return nil
}

func appStatus(c *cli.Context) error {
	appName, err := getAppArg(c)
	if err != nil {
		return err
	}

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}
	kclient := ac.kubernetesClient()

	app, err := resources.GetAppByName(kclient, appName)
	if err != nil {
		return err
	}

	// find all the app targets
	requiredTarget := c.String("target")
	// what information is useful here?
	// group by target
	// Build, date deployed, status, numAvailable/Desired, traffic
	for _, target := range app.Spec.Targets {
		if requiredTarget != "" && target.Name != requiredTarget {
			continue
		}

		fmt.Printf("\nTarget: %s\n", target.Name)

		at, err := resources.GetAppTargetWithLabels(kclient, app.Name, target.Name)
		if err == resources.ErrNotFound {
			fmt.Println("skipping, not configured for the active cluster")
			continue
		}

		// find all targets of this app
		releases, err := resources.GetAppReleases(kclient, app.Name, target.Name, 100)
		if err != nil {
			return err
		}

		// find ingress requests
		ir, err := resources.GetIngressRequestForAppTarget(kclient, app.Name, target.Name)
		if err != nil {
			if err == resources.ErrNotFound {
				// just skip
			} else {
				return err
			}
		}

		if ir != nil {
			fmt.Printf("Hosts: %s\n", strings.Join(ir.Spec.Hosts, ", "))
			fmt.Printf("Load Balancer: %s\n", ir.Status.Address)
		}
		fmt.Printf("Scale: %d min, %d max\n", at.Spec.Scale.Min, at.Spec.Scale.Max)
		fmt.Println()
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{
			"Release", "Build", "Date", "Pods", "Status", "Traffic",
		})
		for _, release := range releases {
			// loading build
			build, err := resources.GetBuildByName(kclient, release.Spec.Build)
			if err != nil {
				return err
			}
			vals := []string{
				release.Name,
				build.ShortName(),
				release.GetCreationTimestamp().Format("2006-01-02 15:04:05"),
				fmt.Sprintf("%d/%d", release.Status.NumAvailable, release.Status.NumDesired),
				release.Status.State.String(),
				fmt.Sprintf("%d%%", release.Spec.TrafficPercentage),
			}
			table.Append(vals)
		}
		table.SetBorder(false)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetColMinWidth(0, 30)
		table.SetColMinWidth(1, 27)
		table.SetColMinWidth(2, 25)
		table.SetColMinWidth(3, 8)
		table.SetColMinWidth(4, 10)
		table.SetCenterSeparator(" ")
		table.SetColumnSeparator(" ")
		table.SetHeaderLine(false)
		table.SetNoWhiteSpace(true)
		table.Render()
		fmt.Println()
	}
	return nil
}

func appDeploy(c *cli.Context) error {
	appName, err := getAppArg(c)
	if err != nil {
		return err
	}

	tag := c.String("tag")

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}
	kclient := ac.kubernetesClient()

	targets, err := resources.GetAppTargets(kclient, appName)
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		return fmt.Errorf("No targets found")
	}

	appTarget := funk.Head(targets).(v1alpha1.AppTarget)
	build, err := resources.GetBuildByName(kclient, appTarget.Spec.Build)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if err == nil {
		return fmt.Errorf("Build %s already exists", build.ShortName())
	}

	// create new build
	build = v1alpha1.NewBuild(appTarget.Labels[resources.BuildRegistryLabel], appTarget.Labels[resources.BuildImageLabel], tag)
	build.Labels[resources.BuildTypeLabel] = resources.BuildTypeLatest

	_, err = resources.UpdateResource(kclient, build, nil, nil)
	return err
}

type appInfo struct {
	AppName     string
	DockerImage string
	DockerTag   string
	Target      string
	Port        int
}

func appNew(c *cli.Context) error {
	ai, err := promptAppInfo()
	if err != nil {
		return err
	}

	box := utils.DeployResourcesBox()
	f, err := box.Open("templates/app.yaml")
	if err != nil {
		return err
	}
	defer f.Close()
	content, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	tmpl, err := template.New("app").Parse(string(content))
	if err != nil {
		return err
	}

	output, err := os.Create(fmt.Sprintf("%s.yaml", ai.AppName))
	if err != nil {
		return err
	}
	defer output.Close()

	err = tmpl.Execute(output, ai)
	if err != nil {
		return err
	}

	fmt.Printf("Your app config has been successfully created: %s\n", output.Name())

	return nil
}

func appLoad(c *cli.Context) error {
	app, err := getAppArg(c)
	if err != nil {
		return err
	}

	_, err = getActiveCluster()
	if err != nil {
		return err
	}

	err = utilscli.KubeApply(app)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully loaded %s\n", app)
	return nil
}

func appEdit(c *cli.Context) error {
	appName, err := getAppArg(c)
	if err != nil {
		return err
	}

	ac, err := getActiveCluster()
	if err != nil {
		return err
	}
	kclient := ac.kubernetesClient()

	app, err := resources.GetAppByName(kclient, appName)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(nil)
	err = kube.GetKubeEncoder().Encode(app, buf)
	if err != nil {
		return err
	}

	// now edit the thing
	data, err := utilscli.ExecuteUserEditor(buf.Bytes(), fmt.Sprintf("%s.yaml", appName))
	if err != nil {
		return err
	}

	obj, _, err := kube.GetKubeDecoder().Decode(data, nil, app)
	if err != nil {
		return err
	}

	app = obj.(*v1alpha1.App)
	op, err := resources.UpdateResource(kclient, app, nil, nil)
	if err != nil {
		return err
	}

	if op == controllerutil.OperationResultNone {
		fmt.Println("App was not changed")
	} else {
		fmt.Println("App updated")
	}
	return nil
}

func appLocal(c *cli.Context) error {
	if c.NArg() < 2 {
		return cli.ShowSubcommandHelp(c)
	}
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	kclient := ac.kubernetesClient()

	pc, err := chooseReleaseHelper(kclient, c)
	if err != nil {
		return err
	}

	release, err := resources.GetAppRelease(kclient, pc.app, pc.target, pc.release)
	if err != nil {
		return err
	}

	// find the config map
	var cm *corev1.ConfigMap
	if release.Spec.Config != "" {
		cm, err = resources.GetConfigMap(kclient, release.Namespace, release.Spec.Config)
		if err != nil {
			return err
		}
	}

	args := c.Args().Slice()[1:]
	fmt.Printf("Running %s...\n", strings.Join(args, " "))
	var cmdArgs []string
	if len(args) > 1 {
		cmdArgs = args[1:]
	}
	cmd := exec.Command(args[0], cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if cm != nil {
		for key, val := range cm.Data {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, utilscli.EscapeEnvVar(val)))
		}
	}

	return cmd.Run()
}

func appLogs(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}
	kclient := ac.kubernetesClient()

	pc, err := choosePodHelper(kclient, c)
	if err != nil {
		return err
	}

	follow := c.Bool("follow")
	verb := "getting"
	if follow {
		verb = "following"
	}
	fmt.Printf("%s logs for pod %s\n", verb, pc.pod)
	namespace := resources.NamespaceForAppTarget(pc.app, pc.target)
	args := []string{
		"logs", pc.pod, "-n", namespace, "-c", "app",
	}
	if c.Bool("follow") {
		args = append(args, "-f")
	}
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func appPods(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	kclient := ac.kubernetesClient()
	pc, err := chooseReleaseHelper(kclient, c)
	if err != nil {
		return err
	}

	pods, err := resources.GetPodsForAppRelease(kclient, resources.NamespaceForAppTarget(pc.app, pc.target), pc.release)
	if err != nil {
		return err
	}

	fmt.Printf("Total pods: %d\n", len(pods))
	for _, p := range pods {
		fmt.Println(p)
	}

	return nil
}

func appRollback(c *cli.Context) error {
	app, err := getAppArg(c)
	if err != nil {
		return err
	}
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}

	kclient := ac.kubernetesClient()

	target := c.String("target")
	release := c.String("release")
	if target == "" {
		if target, err = selectAppTarget(kclient, app); err != nil {
			return err
		}
	}

	if release == "" {
		releases, err := resources.GetAppReleases(kclient, app, target, 10)
		if err != nil {
			return err
		}

		names := funk.Map(releases, func(ar *v1alpha1.AppRelease) string {
			return ar.Name
		})
		prompt := utils.NewPromptSelect("Select a release to mark as bad", names)
		prompt.Size = 10
		_, release, err = prompt.Run()
		if err != nil {
			return err
		}
	}

	ar, err := resources.GetAppRelease(kclient, app, target, release)
	if err != nil {
		return err
	}

	// explicit confirmation
	err = utils.ExplicitConfirmationPrompt(fmt.Sprintf("Do you want to roll back %s?", ar.Name))
	if err != nil {
		return err
	}

	ar.Spec.Role = v1alpha1.ReleaseRoleBad
	_, err = resources.UpdateResource(kclient, ar, nil, nil)
	if err != nil {
		return err
	}

	fmt.Println("Successfully rolled back release")
	return nil
}

func appShell(c *cli.Context) error {
	ac, err := getActiveCluster()
	if err != nil {
		return err
	}
	kclient := ac.kubernetesClient()

	pc, err := choosePodHelper(kclient, c)
	if err != nil {
		return err
	}

	fmt.Printf("initializing shell to pod %s\n", pc.pod)
	namespace := resources.NamespaceForAppTarget(pc.app, pc.target)
	cmd := exec.Command("kubectl", "exec", "-n", namespace, "-it", pc.pod, "--container", "app", "--", c.String("shell"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func getAppArg(c *cli.Context) (string, error) {
	if c.NArg() == 0 {
		cli.ShowSubcommandHelp(c)
		return "", fmt.Errorf("required arg <app> was not passed in")
	}
	return c.Args().Get(0), nil
}

type podContext struct {
	app     string
	target  string
	release string
	pod     string
}

func chooseReleaseHelper(kclient client.Client, c *cli.Context) (pc *podContext, err error) {
	app, err := getAppArg(c)
	if err != nil {
		return
	}

	// if pod is passed in, go straight to that
	target := c.String("target")
	release := c.String("release")
	if target == "" {
		if target, err = selectAppTarget(kclient, app); err != nil {
			return
		}
	}
	if release == "" {
		// find active release
		ar, err := resources.GetActiveRelease(kclient, app, target)
		if err != nil {
			return nil, err
		}
		if ar == nil {
			return nil, fmt.Errorf("Could not find an active release")
		}
		release = ar.Name
	}

	pc = &podContext{
		app:     app,
		target:  target,
		release: release,
	}
	return
}

// helper function to select a pod based on user input
func choosePodHelper(kclient client.Client, c *cli.Context) (pc *podContext, err error) {
	pod := c.String("pod")
	// TODO: if exact pod name is passed in, we could perform a search to find it in any namespace

	pc, err = chooseReleaseHelper(kclient, c)
	if err != nil {
		return
	}

	if pod == "" {
		pod, err = selectAppPod(kclient, pc.app, pc.target, pc.release)
		if err != nil {
			return nil, err
		}
	}

	pc.pod = pod
	return
}

func selectAppTarget(kclient client.Client, appName string) (target string, err error) {
	targets, err := resources.GetAppTargets(kclient, appName)
	if err != nil {
		return
	}

	if len(targets) == 0 {
		err = fmt.Errorf("The app doesn't have any targets deployed on this cluster")
		return
	}
	if len(targets) == 1 {
		target = targets[0].Spec.Target
		return
	}

	// otherwise prompt
	targetNames := funk.Map(targets, func(t v1alpha1.AppTarget) string {
		return t.Name
	})

	prompt := utils.NewPromptSelect("Select a target", targetNames)
	idx, _, err := prompt.Run()
	if err != nil {
		return
	}

	target = targets[idx].Spec.Target
	return
}

func selectAppPod(kclient client.Client, app, target, release string) (pod string, err error) {
	pods, err := resources.GetPodsForAppRelease(kclient, resources.NamespaceForAppTarget(app, target), release)

	if len(pods) == 0 {
		err = fmt.Errorf("No pods found")
		return
	}
	if len(pods) == 1 {
		pod = pods[0]
		return
	}

	prompt := utils.NewPromptSelect("Select a pod", pods)
	prompt.Size = 10
	_, pod, err = prompt.Run()
	return
}

func promptAppInfo() (ai *appInfo, err error) {
	prompt := promptui.Prompt{
		Label:    "Enter app name",
		Validate: utils.ValidateKubeName,
	}
	utils.FixPromptBell(&prompt)

	ai = &appInfo{}
	val, err := prompt.Run()
	if err != nil {
		return
	}
	ai.AppName = val

	// docker image
	prompt = promptui.Prompt{
		Label:    "Docker image",
		Validate: utils.ValidateMinLength(3),
	}
	utils.FixPromptBell(&prompt)
	if val, err = prompt.Run(); err != nil {
		return
	}

	// see if tag is provided
	parts := strings.Split(val, ":")
	if len(parts) == 2 {
		ai.DockerImage = parts[0]
		ai.DockerTag = parts[1]
	} else {
		ai.DockerImage = val
		ai.DockerTag = "latest"
	}

	prompt = promptui.Prompt{
		Label:    "Port (that your app runs on)",
		Validate: utils.ValidateInt,
	}
	if val, err = prompt.Run(); err != nil {
		return
	}

	ai.Port = cast.ToInt(val)

	// if connected to kube, load the first target
	ac, err := getActiveCluster()
	if err != nil {
		return ai, nil
	}

	cc, err := resources.GetClusterConfig(ac.kubernetesClient())
	if err != nil {
		return ai, nil
	}

	if len(cc.Spec.Targets) > 0 {
		ai.Target = cc.Spec.Targets[0]
	}
	return
}
