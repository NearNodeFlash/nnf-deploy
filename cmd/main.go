/*
 * Copyright 2021-2025 Hewlett Packard Enterprise Development LP
 * Other additional copyright holders may be indicated within.
 *
 * The entirety of this work is licensed under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 *
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"gopkg.in/yaml.v3"

	"github.com/NearNodeFlash/nnf-deploy/config"
)

// This is the order in which we process the modules on deployment.
var modules = []string{
	"lustre-csi-driver",
	"lustre-fs-operator",
	"dws",
	"nnf-sos",
	"nnf-dm",
}

// The following modules are allowed to be installed via direct reference
// to github with "kubectl apply -k", depending on their settings
// in config/repositories.yaml.
var modulesAllowedRemote = []string{
	"lustre-csi-driver",
	"lustre-fs-operator",
}

type Context struct {
	Debug        bool
	DryRun       bool
	DryRunAlways bool
	Force        bool

	Systems string
	Repos   string
	Daemons string
}

var cli struct {
	Debug        bool   `help:"Enable debug mode."`
	DryRun       bool   `help:"Show what would be run if modifying the target, but really run things that are usually safe on the target."`
	DryRunAlways bool   `help:"DryRun always, never touch the target."`
	Systems      string `name:"systems" default:"config/systems.yaml" help:"path to the systems config file"`
	Repos        string `name:"repos" default:"config/repositories.yaml" help:"path to the repositories config file"`
	Daemons      string `name:"daemons" default:"config/daemons.yaml" help:"path to the daemons config file"`

	Deploy   DeployCmd   `cmd:"" help:"Deploy to current context."`
	Undeploy UndeployCmd `cmd:"" help:"Undeploy from current context."`
	Make     MakeCmd     `cmd:"" help:"Run make [COMMAND] in every repository."`
	Install  InstallCmd  `cmd:"" help:"Install daemons (EXPERIMENTAL)."`
	Init     InitCmd     `cmd:"" help:"Initialize cluster."`
}

func main() {
	ctx := kong.Parse(&cli)
	err := ctx.Run(&Context{Debug: cli.Debug, DryRun: cli.DryRun, DryRunAlways: cli.DryRunAlways, Systems: cli.Systems, Repos: cli.Repos, Daemons: cli.Daemons})
	ctx.FatalIfErrorf(err)
}

type DeployCmd struct {
	Only []string `arg:"" optional:"" name:"only" help:"Only use these repositories"`
}

func (cmd *DeployCmd) Run(ctx *Context) error {
	if ctx.DryRunAlways {
		ctx.DryRun = true
	}
	system, err := loadSystem(ctx.Systems)
	if err != nil {
		return err
	}

	err = runInModules(modules, func(module string) error {

		if shouldSkipModule(ctx, module, cmd.Only) {
			return nil
		}

		if err := createSystemConfigFromSOS(ctx, system, module); err != nil {
			return err
		}

		if err := deployModule(ctx, system, module); err != nil {
			return err
		}

		if err := createDefaultStorageProfile(ctx, module); err != nil {
			return err
		}

		return nil
	})

	return err
}

type UndeployCmd struct {
	Only []string `arg:"" optional:"" name:"only" help:"Only use these repositories"`
}

func (cmd *UndeployCmd) Run(ctx *Context) error {
	if ctx.DryRunAlways {
		ctx.DryRun = true
	}
	system, err := loadSystem(ctx.Systems)
	if err != nil {
		return err
	}

	reversed := make([]string, len(modules))
	for i := range modules {
		reversed[i] = modules[len(modules)-i-1]
	}

	return runInModules(reversed, func(module string) error {

		if shouldSkipModule(ctx, module, cmd.Only) {
			return nil
		}

		if err := deleteSystemConfigFromSOS(ctx, module); err != nil {
			return err
		}

		// Uninstall first to ensure the CRDs, and therefore all related custom
		// resources, are deleted while the controllers are still running.
		if module != "lustre-csi-driver" && module != "nnf-dm" {
			if err := runMakeCommand(ctx, system, module, "uninstall"); err != nil {
				return err
			}
		}

		if err := runMakeCommand(ctx, system, module, "undeploy"); err != nil {
			return err
		}

		return nil
	})
}

type MakeCmd struct {
	Command string   `arg:"" name:"command" help:"Make target."`
	Only    []string `arg:"" optional:"" name:"only" help:"Only use these repositories"`
}

func (cmd *MakeCmd) Run(ctx *Context) error {
	if ctx.DryRunAlways {
		ctx.DryRun = true
	}
	system, err := loadSystem(ctx.Systems)
	if err != nil {
		return err
	}

	return runInModules(modules, func(module string) error {

		if shouldSkipModule(ctx, module, cmd.Only) {
			return nil
		}

		return runMakeCommand(ctx, system, module, cmd.Command)
	})
}

func runMakeCommand(ctx *Context, system *config.System, module string, command string) error {
	fmt.Printf("  Running `make %s` in %s...\n", command, module)

	cmd := exec.Command("make", command)

	overlay, err := getOverlay(ctx, system, module)
	if err != nil {
		return err
	}

	fmt.Print("  Finding Repository...")
	repo, buildConfig, err := config.FindRepository(ctx.Repos, module)
	if err != nil {
		return err
	}
	fmt.Printf(" %s\n", repo.Name)
	for idx := range buildConfig.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", buildConfig.Env[idx].Name, buildConfig.Env[idx].Value))
	}

	if len(overlay) != 0 {
		cmd.Env = append(cmd.Env,
			"OVERLAY="+overlay,
		)
	}

	_, err = runCommand(ctx, cmd)
	return err
}

type InstallCmd struct {
	Nodes   []string `arg:"" optional:"" name:"node" help:"Only use these nodes"`
	NoBuild bool     `help:"Skip building the daemon"`
	Force   bool     `help:"Force updates even if files are the same"`
}

func (dcmd *InstallCmd) Run(ctx *Context) error {
	if ctx.DryRunAlways {
		ctx.DryRun = true
	}
	ctx.Force = dcmd.Force

	shouldSkipNode := func(node string) bool {
		if len(dcmd.Nodes) == 0 {
			return false
		}

		for _, n := range dcmd.Nodes {
			if n == node {
				return false
			}
		}

		return true
	}

	system, err := loadSystem(ctx.Systems)
	if err != nil {
		return err
	}

	sysConfigCR, err := config.ReadSystemConfigurationCR("config/" + system.SystemConfiguration)
	if err != nil {
		return err
	}
	perRabbit := sysConfigCR.RabbitsAndComputes()
	externalComputes := sysConfigCR.ExternalComputes()

	clusterConfig, err := currentClusterConfig()
	if err != nil {
		return err
	}

	fmt.Println("Found Cluster Configuration:", clusterConfig)

	clusterConfig = strings.TrimPrefix(clusterConfig, "https://")

	k8sServerHost := clusterConfig[:strings.Index(clusterConfig, ":")]
	k8sServerPort := clusterConfig[strings.Index(clusterConfig, ":")+1:]

	// Let the config override these values pulled from the cluster config. The values are used for
	// daemons on the compute nodes, which may need a different IP/network to hit the cluster than
	// the public facing cluster IP that the cluster config is using.
	if system.K8sHost != "" {
		k8sServerHost = system.K8sHost
	}
	if system.K8sPort != "" {
		k8sServerPort = system.K8sPort
	}

	return config.EnumerateDaemons(ctx.Daemons, func(d config.Daemon) error {

		var token []byte
		var cert []byte
		if d.ServiceAccount.Name != "" {
			fmt.Println("\nLoading Service Account Cert & Token")

			fmt.Println("  Secret:", d.ServiceAccount.Name+"/"+d.ServiceAccount.Namespace)

			fmt.Printf("  Token...")
			token, err = exec.Command("bash", "-c", fmt.Sprintf("kubectl get secret %s -n %s -o json | jq -Mr '.data.token' | base64 --decode", d.ServiceAccount.Name, d.ServiceAccount.Namespace)).Output()
			if err != nil {
				return err
			}
			fmt.Println("Loaded REDACTED")

			fmt.Printf("  Cert...")
			cert, err = exec.Command("bash", "-c", fmt.Sprintf("kubectl get secret %s -n %s -o json | jq -Mr '.data.\"ca.crt\"' | base64 --decode", d.ServiceAccount.Name, d.ServiceAccount.Namespace)).Output()
			if err != nil {
				return err
			}
			fmt.Println("Loaded REDACTED")
		}

		err = runInModules([]string{d.Repository}, func(module string) error {

			fmt.Printf("\nChecking module %s\n\n", module)

			if d.Bin != "" && d.BuildCmd != "" {
				b := strings.Fields(d.BuildCmd)
				cmd := exec.Command(b[0], b[1:]...)

				fmt.Printf("Compile %s daemon...\n", d.Bin)
				if dcmd.NoBuild {
					fmt.Printf("  No-Build: %s\n", cmd.String())
				} else {
					if _, err := runSafeCommand(ctx, cmd); err != nil {
						return err
					}
					fmt.Printf("DONE\n")
				}
			}

			// Change to the bin's output path
			if d.Path != "" {
				fmt.Printf("  Chdir %s\n", d.Path)
				if err := os.Chdir(d.Path); err != nil {
					return err
				}
			}

			installCompute := func(compute string) error {
				fmt.Printf("\n Checking for install on Compute Node %s\n\n", compute)

				if shouldSkipNode(compute) {
					return nil
				}

				fmt.Printf("  Installing %s on Compute Node %s\n", d.Name, compute)

				configDir := "/etc/" + d.Name
				if len(token) != 0 || len(cert) != 0 {
					cmd := exec.Command("ssh", compute, "mkdir -p "+configDir)
					if _, err := runCommand(ctx, cmd); err != nil {
						return err
					}
				}

				serviceTokenPath := configDir
				tokenNeedsUpdate := false
				if len(token) != 0 {
					if err := os.WriteFile("service.token", token, 0644); err != nil {
						return err
					}

					tokenNeedsUpdate, err = checkNeedsUpdate(ctx, "service.token", compute, serviceTokenPath)
					if tokenNeedsUpdate {
						err = copyToNode(ctx, "service.token", compute, serviceTokenPath)
					}

					os.Remove("service.token")

					if err != nil {
						return err
					}
				}

				certFilePath := configDir
				certNeedsUpdate := false
				if len(cert) != 0 {
					if err := os.WriteFile("service.cert", cert, 0644); err != nil {
						return err
					}

					certNeedsUpdate, err = checkNeedsUpdate(ctx, "service.cert", compute, certFilePath)
					if certNeedsUpdate {
						err = copyToNode(ctx, "service.cert", compute, certFilePath)
					}

					os.Remove("service.cert")

					if err != nil {
						return err
					}
				}

				if d.Bin == "" {
					return nil
				}

				binaryNeedsUpdate, err := checkNeedsUpdate(ctx, d.Bin, compute, "/usr/bin")
				if err != nil {
					return err
				}

				if binaryNeedsUpdate {

					fmt.Printf("  Stopping %s service...", d.Name)
					cmd := exec.Command("ssh", compute, "systemctl", "stop", d.Bin, "|| true")
					if _, err := runCommand(ctx, cmd); err != nil {
						return err
					}
					fmt.Printf("\n")

					fmt.Printf("  Removing %s service...", d.Name)
					cmd = exec.Command("ssh", compute, "/usr/bin/"+d.Bin, "remove", "|| true")
					if _, err := runCommand(ctx, cmd); err != nil {
						return err
					}
					fmt.Printf("\n")

					if err := copyToNode(ctx, d.Bin, compute, "/usr/bin"); err != nil {
						return err
					}

					fmt.Printf("  Installing %s service...", d.Name)
					cmd = exec.Command("ssh", compute, "/usr/bin/"+d.Bin, "install", "|| true")
					if _, err := runCommand(ctx, cmd); err != nil {
						return err
					}
					fmt.Printf("\n")
				}

				overrideContents := ""
				overrideContents += "[Service]\n"
				overrideContents += "ExecStart=\n"
				overrideContents += "ExecStart=/usr/bin/" + d.Bin + " \\\n"
				overrideContents += "  --kubernetes-service-host=" + k8sServerHost + " \\\n"
				overrideContents += "  --kubernetes-service-port=" + k8sServerPort

				// optional command line arguments
				if len(token) != 0 {
					overrideContents += " \\\n" + "  --service-token-file=" + path.Join(serviceTokenPath, "service.token")
				}
				if len(cert) != 0 {
					overrideContents += " \\\n" + "  --service-cert-file=" + path.Join(certFilePath, "service.cert")
				}
				if len(d.ExtraArgs) > 0 {
					overrideContents += " \\\n  " + d.ExtraArgs
				}

				// Add environment variables - there should not be a \ on the preceding line
				// otherwise the first env var will not work
				for _, e := range d.Environment {
					overrideContents += "\n" + "Environment=" + e.Name + "=" + e.Value
				}

				fmt.Printf("  Creating override directory...")
				overridePath := "/etc/systemd/system/" + d.Bin + ".service.d"
				cmd := exec.Command("ssh", compute, "mkdir", "-p", overridePath)
				if _, err := runCommand(ctx, cmd); err != nil {
					return err
				}
				fmt.Printf("\n")

				fmt.Println("  Creating override configuration...")
				if err := os.WriteFile("override.conf", []byte(overrideContents), 0644); err != nil {
					return err
				}

				overrideNeedsUpdate, err := checkNeedsUpdate(ctx, "override.conf", compute, overridePath)
				if overrideNeedsUpdate {
					err = copyToNode(ctx, "override.conf", compute, overridePath)
				}

				os.Remove("override.conf")

				if err != nil {
					return err
				}

				if binaryNeedsUpdate || tokenNeedsUpdate || certNeedsUpdate || overrideNeedsUpdate {
					// Reload the daemon to pick up the override.conf.
					fmt.Printf("  Reloading service...")
					cmd = exec.Command("ssh", compute, "systemctl daemon-reload")
					if _, err := runCommand(ctx, cmd); err != nil {
						return err
					}
					fmt.Printf("\n")

					fmt.Printf("  Starting service...")
					cmd = exec.Command("ssh", compute, "systemctl", "start", d.Bin)
					if _, err := runCommand(ctx, cmd); err != nil {
						return err
					}
					fmt.Printf("\n")
				}

				return nil
			}

			for rabbit, computes := range perRabbit {
				fmt.Printf("\n Check clients of rabbit %s\n\n", rabbit)

				for _, compute := range computes {
					err := installCompute(compute)
					if err != nil {
						return err
					}
				}
			}

			for _, compute := range externalComputes {
				fmt.Printf("\n Check external computes\n\n")

				err := installCompute(compute)
				if err != nil {
					return err
				}
			}

			return nil
		})

		return err
	})
}

type InitCmd struct{}

func (cmd *InitCmd) Run(ctx *Context) error {
	if ctx.DryRunAlways {
		ctx.DryRun = true
	}
	system, err := loadSystem(ctx.Systems)
	if err != nil {
		return err
	}

	if err := installThirdPartyServices(ctx); err != nil {
		return err
	}

	for _, module := range modulesAllowedRemote {
		var applyK string
		repo, _, err := config.FindRepository(ctx.Repos, module)
		if err != nil {
			return err
		}
		if !repo.UseRemoteK {
			continue
		}
		fmt.Printf("Installing %s...\n", module)
		overlay, err := getOverlay(ctx, system, module)
		if err != nil {
			return err
		}
		if overlay == "" {
			applyK = fmt.Sprintf(repo.RemoteReference.Url, repo.RemoteReference.Build)
		} else {
			applyK = fmt.Sprintf(repo.RemoteReference.Url, overlay, repo.RemoteReference.Build)
		}
		if err := runKubectlApplyK(ctx, applyK); err != nil {
			return err
		}
	}

	return nil
}

func installThirdPartyServices(ctx *Context) error {
	thirdPartyServices, err := config.GetThirdPartyServices(ctx.Repos)
	if err != nil {
		return err
	}

	for idx := range thirdPartyServices {
		svc := thirdPartyServices[idx]
		if svc.UseRemoteF {
			fmt.Printf("Installing %s...\n", svc.Name)
			if err := runKubectlApplyF(ctx, svc.Url); err != nil {
				return err
			}
		} else if svc.UseHelm {
			fmt.Printf("Installing %s...\n", svc.Name)
			cmd := exec.Command("bash", "-c", svc.HelmCmd)
			_, err = runCommand(ctx, cmd)
			if err != nil {
				return err
			}
		} else {
			continue
		}
		if len(svc.WaitCmd) > 0 {
			fmt.Println("  waiting for it to be ready...")
			cmd := exec.Command("bash", "-c", svc.WaitCmd)
			_, err = runCommand(ctx, cmd)
			if err != nil {
				fmt.Printf("\n\033[1mThe cluster is still waiting for %s to start.\nPlease run `nnf-deploy init` after it is running.\033[0m\n\n", svc.Name)
				return err
			}
		}
	}
	return nil
}

func runKubectlApply(ctx *Context, applyFlag string, url string) error {
	cmd := exec.Command("kubectl", "apply", applyFlag, url)
	if _, err := runCommand(ctx, cmd); err != nil {
		return err
	}
	return nil
}

func runKubectlApplyK(ctx *Context, url string) error {
	return runKubectlApply(ctx, "-k", url)
}

func runKubectlApplyF(ctx *Context, url string) error {
	return runKubectlApply(ctx, "-f", url)
}

type k8sCluster struct {
	Name    string
	Cluster struct {
		Server string
	}
}

type k8sContext struct {
	Name    string
	Context struct {
		Cluster string
	}
}
type k8sConfig struct {
	Kind     string
	Contexts []k8sContext
	Clusters []k8sCluster
}

func currentClusterConfig() (string, error) {
	current, err := exec.Command("kubectl", "config", "current-context").Output()
	if err != nil {
		return "", err
	}
	current = current[:len(current)-1]

	configView, err := exec.Command("kubectl", "config", "view").Output()
	if err != nil {
		return "", err
	}

	config := new(k8sConfig)
	if err := yaml.Unmarshal(configView, config); err != nil {
		return "", err
	}

	currentContext := string(current)
	for _, context := range config.Contexts {
		if context.Name == currentContext {
			for _, cluster := range config.Clusters {
				if cluster.Name == context.Context.Cluster {
					return cluster.Cluster.Server, nil
				}
			}

			return "", fmt.Errorf("cluster Name '%s' not found", context.Context.Cluster)
		}
	}

	return "", fmt.Errorf("current Context '%s' not found", currentContext)
}

func checkNeedsUpdate(ctx *Context, name string, compute string, destination string) (bool, error) {
	fmt.Printf("  Checking Compute Node %s needs update to %s...\n", compute, name)

	if ctx.Force {
		fmt.Printf("    Update forced by --force option\n")
		return true, nil
	}

	compareMD5 := func(first []byte, second []byte) bool {
		if len(second) < len(first) {
			return false
		}

		for i := 0; i < len(first); i++ {
			if first[i] == ' ' {
				return true
			}

			if first[i] != second[i] {
				return false
			}
		}

		return false
	}

	fmt.Printf("    Source MD5: ")
	src, err := runSafeCommand(ctx, exec.Command("md5sum", name))
	if err != nil {
		return false, err
	}
	fmt.Printf("%s", src)

	fmt.Printf("    Destination MD5: ")
	dest, err := runSafeCommand(ctx, exec.Command("ssh", "-q", compute, "md5sum "+path.Join(destination, name), " || true"))
	if err != nil {
		return false, err
	}
	fmt.Printf("%s", dest)

	needsUpdate := !compareMD5(src, dest)
	if needsUpdate {
		fmt.Printf("  Compute Node %s requires update to %s\n", compute, name)
	}

	if needsUpdate && ctx.DryRun {
		needsUpdate = false
		fmt.Printf("  Dry-Run: Skipping update of '%s'\n", name)
	}

	return needsUpdate, nil
}

func copyToNode(ctx *Context, name string, compute string, destination string) error {
	fmt.Printf("  Copying %s to %s at %s...", name, compute, destination)
	if _, err := runCommand(ctx, exec.Command("scp", "-OC", name, compute+":"+destination)); err != nil {
		return err
	}

	fmt.Printf("\n")
	return nil
}

func currentContext() (string, error) {
	out, err := exec.Command("kubectl", "config", "current-context").Output()
	return strings.TrimRight(string(out), "\r\n"), err
}

func loadSystem(configPath string) (*config.System, error) {
	fmt.Println("Retrieving Context...")
	ctx, err := currentContext()
	if err != nil {
		return nil, err
	}

	fmt.Println("Retrieving System Config...")
	system, err := config.FindSystem(ctx, configPath)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Target System: %s\n", system.Name)
	return system, nil
}

func currentBranch() (string, error) {
	out, err := exec.Command("git", "branch", "--show-current").Output()
	return strings.TrimRight(string(out), "\r\n"), err
}

func lastLocalCommit() (string, error) {
	out, err := exec.Command("./git-version-gen").Output()
	return strings.TrimRight(string(out), "\r\n"), err
}

func getOverlay(ctx *Context, system *config.System, module string) (string, error) {

	repo, _, err := config.FindRepository(ctx.Repos, module)
	if err != nil {
		return "", err
	}

	for _, repoOverlay := range repo.Overlays {
		for _, systemOverlay := range system.Overlays {
			if repoOverlay == systemOverlay {
				fmt.Printf("  Overlay for %s found: %s\n", module, repoOverlay)
				return repoOverlay, nil
			}
		}
	}

	return "", nil
}

func getExampleOverlay(ctx *Context, system *config.System, module string) (string, error) {

	repo, _, err := config.FindRepository(ctx.Repos, module)
	if err != nil {
		return "", err
	}

	for _, repoOverlay := range repo.Overlays {
		for _, systemOverlay := range system.Overlays {
			if repoOverlay == systemOverlay && strings.HasPrefix(systemOverlay, "examples-") {
				fmt.Printf("  Examples Overlay for %s found: %s\n", module, repoOverlay)
				return repoOverlay, nil
			}
		}
	}

	return "", nil
}

func deployModule(ctx *Context, system *config.System, module string) error {

	cmd := exec.Command("make", "deploy")

	overlay, err := getOverlay(ctx, system, module)
	if err != nil {
		return err
	}

	// Some repos apply examples (e.g. nnf-sos' container/storage profiles) in an additional step in
	// deploy.sh, so account for an additional overlay to use in that case.
	overlayExample, err := getExampleOverlay(ctx, system, module)
	if err != nil {
		return err
	}

	fmt.Print("  Finding Repository...")
	repo, buildConfig, err := config.FindRepository(ctx.Repos, module)
	if err != nil {
		return err
	}
	fmt.Printf(" %s\n", repo.Name)
	for idx := range buildConfig.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", buildConfig.Env[idx].Name, buildConfig.Env[idx].Value))
	}

	fmt.Printf("  Loading Current Branch...")
	branch, err := currentBranch()
	if err != nil {
		return err
	}
	// For the detached head case, i.e. there is no branch name, assume
	// that at some point that commit was built in the master branch
	// and treat it like the 'master' branch case.
	if branch == "" {
		branch = "master"
	}
	fmt.Printf(" %s\n", branch)

	url := repo.Master
	if branch != "master" {
		url = repo.Development
	}

	fmt.Printf("  Loading Last Commit...")
	commit, err := lastLocalCommit()
	if err != nil {
		return err
	}
	fmt.Printf(" %s\n", commit)

	version := commit
	imageTagBase := strings.TrimSuffix(strings.TrimPrefix(url, "https://"), "/") // According to Tony; docker assumes a secure repo and prepends https when it fetches the image; so we drop it here.

	cmd.Env = append(cmd.Env,
		"IMAGE_TAG_BASE="+imageTagBase,
		"VERSION="+version,
		"OVERLAY="+overlay,
	)

	if len(overlayExample) > 0 {
		cmd.Env = append(cmd.Env, "OVERLAY_EXAMPLES="+overlayExample)
	}

	fmt.Println("  Running Deploy...")
	_, err = runCommand(ctx, cmd)
	return err
}

func runCommand(ctx *Context, cmd *exec.Cmd) ([]byte, error) {
	return runCommandErrAllowed(ctx, cmd, false)
}

func runSafeCommand(ctx *Context, cmd *exec.Cmd) ([]byte, error) {
	savedDryRun := ctx.DryRun
	if !ctx.DryRunAlways {
		// We're allowed to really run this "safe" command.
		ctx.DryRun = false
	}
	out, err := runCommandErrAllowed(ctx, cmd, false)
	ctx.DryRun = savedDryRun
	return out, err
}

func runCommandErrAllowed(ctx *Context, cmd *exec.Cmd, errAllowed bool) ([]byte, error) {
	if ctx.DryRun {
		fmt.Printf("  Dry-Run: %s\n", cmd.String())
		if len(cmd.Env) > 0 {
			fmt.Printf("  Additional env: %v\n", cmd.Env)
		}
		return nil, nil
	}

	if len(cmd.Env) > 0 {
		cmd.Env = append(cmd.Env, os.Environ()...)
	}
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		if !errAllowed {
			fmt.Printf("%s\n", stdoutStderr)

			exitErr := &exec.ExitError{}
			if errors.As(err, &exitErr) {
				fmt.Printf("Exit Error: %s (%d)\n", exitErr, exitErr.ExitCode())
			}
		}
	}

	return stdoutStderr, err
}

func runInModules(modules []string, runFn func(module string) error) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	for _, module := range modules {

		if err := os.Chdir(module); err != nil {
			return err
		}

		err = runFn(module)
		os.Chdir(cwd)

		if err != nil {
			return err
		}
	}

	return nil
}

func shouldSkipModule(ctx *Context, module string, permittedModulesOrEmpty []string) bool {
	// Modules that are being installed via remote should be skipped.
	for _, remoteModule := range modulesAllowedRemote {
		if module == remoteModule {
			repo, _, err := config.FindRepository(ctx.Repos, module)
			if err != nil {
				return true
			}
			if repo.UseRemoteK {
				return true
			}
			break
		}
	}

	if len(permittedModulesOrEmpty) == 0 {
		return false
	}

	for _, permittedModule := range permittedModulesOrEmpty {
		if strings.Contains(module, permittedModule) {
			return false
		}
	}

	return true
}

func deleteSystemConfigFromSOS(ctx *Context, module string) error {
	if !strings.Contains(module, "nnf-sos") {
		return nil
	}

	// Check if the SystemConfiguration resource exists, and return if it doesn't
	getCmd := exec.Command("kubectl", "get", "systemconfiguration", "default", "--no-headers")
	stdoutStderr, err := runCommandErrAllowed(ctx, getCmd, true)
	if strings.Contains(string(stdoutStderr), "could not find") {
		return nil
	} else if err != nil {
		fmt.Printf("%s\n", stdoutStderr)
	}

	fmt.Println("Deleting SystemConfiguration")
	deleteCmd := exec.Command("kubectl", "delete", "systemconfiguration", "default")

	if _, err := runCommand(ctx, deleteCmd); err != nil {
		return err
	}

	// Wait until the SystemConfiguration resource is completely gone.
	for {
		if _, err := runCommand(ctx, getCmd); err != nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	return nil
}

// createSystemConfigFromSOS creates a DWS SystemConfiguration resource using
// information found in the systems.yaml file.
func createSystemConfigFromSOS(ctx *Context, system *config.System, module string) error {
	if !strings.Contains(module, "nnf-sos") {
		return nil
	}

	fmt.Println("Applying SystemConfiguration...")

	cmd := exec.Command("kubectl", "create", "-f", "../config/"+system.SystemConfiguration)
	stdoutStderr, err := runCommandErrAllowed(ctx, cmd, true)
	if strings.Contains(string(stdoutStderr), "already exists") {
		return nil
	} else {
		fmt.Printf("%s\n", stdoutStderr)
	}
	return err
}

// createDefaultStorageProfile creates the default NnfStorageProfile.
func createDefaultStorageProfile(ctx *Context, module string) error {
	if !strings.Contains(module, "nnf-sos") {
		return nil
	}

	fmt.Println("Creating default NnfStorageProfile...")

	cmd := exec.Command("../tools/default-nnfstorageprofile.sh")
	if _, err := runCommand(ctx, cmd); err != nil {
		return err
	}

	return nil
}
