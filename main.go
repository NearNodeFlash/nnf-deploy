/*
 * Copyright 2021, 2022 Hewlett Packard Enterprise Development LP
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
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"gopkg.in/yaml.v2"

	dwsv1alpha1 "github.com/HewlettPackard/dws/api/v1alpha1"
	"github.com/NearNodeFlash/nnf-deploy/config"
)

// This is the order in which we process the modules on deployment.
var modules = []string{
	"dws",
	"lustre-csi-driver",
	"lustre-fs-operator",
	"nnf-sos",
	"nnf-dm",
}

type Context struct {
	Debug  bool
	DryRun bool
	Force  bool
}

var cli struct {
	Debug  bool `help:"Enable debug mode."`
	DryRun bool `help:"Show what would be run."`

	Deploy   DeployCmd   `cmd:"" help:"Deploy to current context."`
	Undeploy UndeployCmd `cmd:"" help:"Undeploy from current context."`
	Make     MakeCmd     `cmd:"" help:"Run make [COMMAND] in every repository."`
	Install  InstallCmd  `cmd:"" help:"Install daemons (EXPERIMENTAL)."`
	Init     InitCmd     `cmd:"" help:"Initialize cluster."`
	Release  ReleaseCmd  `cmd:"" help:"Create or use a release."`
}

func main() {
	ctx := kong.Parse(&cli)
	err := ctx.Run(&Context{Debug: cli.Debug, DryRun: cli.DryRun})
	if err != nil {
		fmt.Printf("%v", err)
	}
	ctx.FatalIfErrorf(err)
}

type DeployCmd struct {
	Only []string `arg:"" optional:"" name:"only" help:"Only use these repositories"`
}

func (cmd *DeployCmd) Run(ctx *Context) error {
	system, err := loadSystem()
	if err != nil {
		return err
	}

	err = runInModules(modules, func(module string) error {

		if shouldSkipModule(module, cmd.Only) {
			return nil
		}

		if err := deployModule(ctx, system, module); err != nil {
			return err
		}

		if err := createSystemConfigFromSOS(ctx, system, module); err != nil {
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
	system, err := loadSystem()
	if err != nil {
		return err
	}

	reversed := make([]string, len(modules))
	for i := range modules {
		reversed[i] = modules[len(modules)-i-1]
	}

	return runInModules(reversed, func(module string) error {

		if shouldSkipModule(module, cmd.Only) {
			return nil
		}

		if err := deleteSystemConfigFromSOS(ctx, system, module); err != nil {
			return err
		}

		// Uninstall first to ensure the CRDs, and therefore all related custom
		// resources, are deleted while the controllers are still running.
		if module != "lustre-csi-driver" {
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
	system, err := loadSystem()
	if err != nil {
		return err
	}

	return runInModules(modules, func(module string) error {

		if shouldSkipModule(module, cmd.Only) {
			return nil
		}

		return runMakeCommand(ctx, system, module, cmd.Command)
	})
}

func runMakeCommand(ctx *Context, system *config.System, module string, command string) error {
	fmt.Printf("  Running `make %s` in %s...\n", command, module)

	cmd := exec.Command("make", command)

	overlay, err := getOverlay(system, module)
	if err != nil {
		return err
	}

	if len(overlay) != 0 {
		cmd.Env = append(os.Environ(),
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

func (cmd *InstallCmd) Run(ctx *Context) error {

	ctx.Force = cmd.Force

	shouldSkipNode := func(node string) bool {
		if len(cmd.Nodes) == 0 {
			return false
		}

		for _, n := range cmd.Nodes {
			if n == node {
				return false
			}
		}

		return true
	}

	system, err := loadSystem()
	if err != nil {
		return err
	}

	clusterConfig, err := currentClusterConfig()
	if err != nil {
		return err
	}

	fmt.Println("Found Cluster Configuration:", clusterConfig)

	clusterConfig = strings.TrimPrefix(clusterConfig, "https://")

	k8sServerHost := clusterConfig[:strings.Index(clusterConfig, ":")]
	k8sServerPort := clusterConfig[strings.Index(clusterConfig, ":")+1:]

	return config.EnumerateDaemons(func(d config.Daemon) error {

		var token []byte
		var cert []byte
		if d.ServiceAccount.Name != "" {
			fmt.Println("Loading Service Account Cert & Token")

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

			fmt.Printf("Checking module %s\n", module)

			if d.Path != "" {
				if err := os.Chdir(d.Path); err != nil {
					return err
				}
			}

			if !cmd.NoBuild && d.Bin != "" {
				cmd := exec.Command("go", "build", "-o", d.Bin)
				cmd.Env = append(os.Environ(),
					"CGO_ENABLED=0",
					"GOOS=linux",
					"GOARCH=amd64",
					"GOPRIVATE=github.hpe.com",
				)

				fmt.Printf("Compile %s daemon...", d.Bin)
				if _, err := runCommand(ctx, cmd); err != nil {
					return err
				}
				fmt.Printf("DONE\n")
			}

			for rabbit := range system.Rabbits {
				fmt.Printf(" Check clients of rabbit %s\n", rabbit)

				for _, compute := range system.Rabbits[rabbit] {
					fmt.Printf(" Checking for install on Compute Node %s\n", compute)

					if shouldSkipNode(compute) {
						continue
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
						continue
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

					execStart := ""
					execStart += "[Service]\n"
					execStart += "ExecStart=\n"
					execStart += "ExecStart=/usr/bin/" + d.Bin + " \\\n"
					execStart += "  --kubernetes-service-host=" + k8sServerHost + " \\\n"
					execStart += "  --kubernetes-service-port=" + k8sServerPort + " \\\n"
					execStart += "  --node-name=" + compute + " \\\n"
					if !d.SkipNnfNodeName {
						execStart += "  --nnf-node-name=" + rabbit + " \\\n"
					}
					if len(token) != 0 {
						execStart += "  --service-token-file=" + path.Join(serviceTokenPath, "service.token") + " \\\n"
					}
					if len(cert) != 0 {
						execStart += "  --service-cert-file=" + path.Join(certFilePath, "service.cert") + " \\\n"
					}
					if len(d.ExtraArgs) > 0 {
						execStart += "  " + d.ExtraArgs + " \\\n"
					}

					fmt.Printf("  Creating override directory...")
					overridePath := "/etc/systemd/system/" + d.Bin + ".service.d"
					cmd := exec.Command("ssh", compute, "mkdir", "-p", overridePath)
					if _, err := runCommand(ctx, cmd); err != nil {
						return err
					}
					fmt.Printf("\n")

					fmt.Println("  Creating override configuration...")
					if err := os.WriteFile("override.conf", []byte(execStart), 0644); err != nil {
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
				}
			}

			return nil
		})

		return err
	})
}

type ReleaseCmd struct {
	List   ReleaseListCmd   `cmd:"" name:"list" help:"List the available releases."`
	Info   ReleaseInfoCmd   `cmd:"" name:"info" help:"Information about a single release."`
	Create ReleaseCreateCmd `cmd:"" name:"create" help:"Create a release using the current repository versions."`
	Set    ReleaseSetCmd    `cmd:"" name:"set" help:"Set the release and checkout the correct submodule commits."`
}

type ReleaseListCmd struct{}

func (cmd *ReleaseListCmd) Run(ctx *Context) error {
	manifest, err := ReadReleaseManifest()
	if err != nil {
		return err
	}

	currentVersion := manifest.CurrentVersion
	if currentVersion == "" {
		currentVersion = "[none]"
	}

	fmt.Printf("Current Version: %s\n", currentVersion)
	fmt.Printf("Available Versions:\n")
	for _, release := range manifest.Releases {
		fmt.Printf("%s\n", release.Version)
	}

	return nil
}

type ReleaseInfoCmd struct {
	Version string `name:"version" help:"version of the release to create"`
}

func (cmd *ReleaseInfoCmd) Run(ctx *Context) error {
	manifest, err := ReadReleaseManifest()
	if err != nil {
		return err
	}

	version := cmd.Version
	if version == "" {
		if manifest.CurrentVersion == "" {
			return nil
		}

		version = manifest.CurrentVersion
	}

	// Find the release that matches the specified version
	release := func() *Release {
		for _, release := range manifest.Releases {
			if release.Version == version {
				return &release
			}
		}

		return nil
	}()

	if release == nil {
		return fmt.Errorf("release '%s' not found", cmd.Version)
	}

	// Print the tag and commit information for each component in the release
	fmt.Printf("Version: %s\n", version)
	for _, component := range release.Components {
		fmt.Printf("%s\n", component.Name)
		if component.Tag != "" {
			fmt.Printf("  Tag: %s\n", component.Tag)
		}

		if component.Commit != "" {
			fmt.Printf("  Commit: %s\n", component.Commit)
		}
	}

	if cmd.Version != "" {
		return nil
	}

	// If version wasn't specified, then we're showing information about the current commit. Do
	// some sanity checking to make sure that all the modules are on the right commit.
	checkComponentRelease := func(module string) error {
		for _, component := range release.Components {
			if component.Name != module {
				continue
			}

			if component.Tag != "" {
				tag, err := currentTag()
				if err != nil {
					return err
				}

				if tag != component.Tag {
					fmt.Printf("Mismatch for component %s between expected and actual tag\n", module)
					fmt.Printf("  Version %s expected tag: %s\n", version, component.Tag)
					fmt.Printf("  Actual tag: %s\n", tag)
				}
			} else {
				commit, err := lastLocalCommit()
				if err != nil {
					return err
				}

				if commit != component.Commit {
					fmt.Printf("Mismatch for component %s between expected and actual commit\n", module)
					fmt.Printf("  Version %s expected commit: %s\n", version, component.Commit)
					fmt.Printf("  Actual commit: %s\n", commit)
				}
			}
		}

		return nil
	}

	if err := checkComponentRelease("nnf-deploy"); err != nil {
		return err
	}

	// nnf-deploy might have a different list of "modules" if components were added or deleted. Use
	// the list of components in the release
	var moduleList []string
	for _, component := range release.Components {
		if component.Name != "nnf-deploy" {
			moduleList = append(moduleList, component.Name)
		}
	}

	if err := runInModules(moduleList, checkComponentRelease); err != nil {
		return err
	}

	return nil
}

type ReleaseCreateCmd struct {
	Version string `name:"version" help:"version of the release to create"`
	Tag     bool   `name:"tag" optional:"" help:"create a git tag on the nnf-deploy repo"`
}

func (cmd *ReleaseCreateCmd) Run(ctx *Context) error {
	manifest, err := ReadReleaseManifest()
	if err != nil {
		return err
	}

	// Check if the version already exists in the manifest
	for _, release := range manifest.Releases {
		if release.Version == cmd.Version {
			return fmt.Errorf("version already exists")
		}
	}

	fmt.Printf("Creating version %s\n", cmd.Version)

	// If the tag flag was specified, tag the nnf-deploy repo with the version string
	if cmd.Tag {
		fmt.Printf("Tagging nnf-deploy commit with: '%s'", cmd.Version)

		if err := addTag(cmd.Version); err != nil {
			return err
		}
	}

	// Set the release information in the manifest
	release := new(Release)
	release.Version = cmd.Version

	addComponentRelease := func(module string) error {
		component := new(ReleaseComponent)
		branch, err := currentBranch()
		if err != nil {
			return err
		}

		commit, err := lastLocalCommit()
		if err != nil {
			return err
		}

		url, err := repoURL()
		if err != nil {
			return err
		}

		tag, err := currentTag()
		if err != nil {
			return err
		}

		component.Name = module
		component.Repository = url
		component.Branch = branch

		if tag != "" {
			component.Tag = tag
			fmt.Printf("Finding %s tag... %s\n", module, tag)
		} else {
			component.Commit = commit
			fmt.Printf("Finding %s commit... %s\n", module, commit)
		}

		release.Components = append(release.Components, *component)
		return nil
	}

	if err := addComponentRelease("nnf-deploy"); err != nil {
		return err
	}

	if err := runInModules(modules, addComponentRelease); err != nil {
		return err
	}

	manifest.Releases = append(manifest.Releases, *release)

	// Set the current version in the local manifest file
	manifest.CurrentVersion = release.Version
	if err := WriteReleaseManifest(manifest, ".release_manifest.yaml"); err != nil {
		return err
	}

	// Don't set the current version in the manifest file that gets checked
	// into the git repo.
	manifest.CurrentVersion = ""
	if err := WriteReleaseManifest(manifest, "release_manifest.yaml"); err != nil {
		return err
	}

	return nil
}

type ReleaseSetCmd struct {
	Version string `name:"version" help:"release version to use"`
}

func (cmd *ReleaseSetCmd) Run(ctx *Context) error {
	manifest, err := ReadReleaseManifest()
	if err != nil {
		return err
	}

	// Find the release that matches the specified version
	release := func() *Release {
		for _, release := range manifest.Releases {
			if release.Version == cmd.Version {
				return &release
			}
		}

		return nil
	}()

	if release == nil {
		return fmt.Errorf("release '%s' not found", cmd.Version)
	}

	// Checkout the correct commit for each of the submodules. This puts the submodule in
	// the detached head state
	setComponentRelease := func(module string) error {
		for _, component := range release.Components {
			if component.Name != module {
				continue
			}

			if component.Tag != "" {
				fmt.Printf("Setting %s to tag %s\n", module, component.Tag)

				return checkoutCommit(component.Tag)
			} else {
				fmt.Printf("Setting %s to commit %s\n", module, component.Commit)

				return checkoutCommit(component.Commit)
			}
		}

		return fmt.Errorf("could not find module %s\n", module)
	}

	if err := setComponentRelease("nnf-deploy"); err != nil {
		return err
	}

	// nnf-deploy might have a different list of "modules" if components were added or deleted. Use
	// the list of components in the release
	var moduleList []string
	for _, component := range release.Components {
		if component.Name != "nnf-deploy" {
			moduleList = append(moduleList, component.Name)
		}
	}

	if err := runInModules(moduleList, setComponentRelease); err != nil {
		return err
	}

	manifest.CurrentVersion = release.Version
	if err := WriteReleaseManifest(manifest, ".release_manifest.yaml"); err != nil {
		return err
	}

	return nil
}

type ReleaseComponent struct {
	Repository string `yaml:"repository"`
	Commit     string `yaml:"commit,omitempty"`
	Tag        string `yaml:"tag,omitempty"`
	Branch     string `yaml:"branch,omitempty"`
	Name       string `yaml:"name"`
}

type Release struct {
	Version    string             `yaml:"version"`
	Components []ReleaseComponent `yaml:"components"`
}

type ReleaseManifest struct {
	CurrentVersion string    `yaml:"currentVersion,omitempty"`
	Releases       []Release `yaml:"releases"`
}

func ReadReleaseManifest() (*ReleaseManifest, error) {
	manifestFile, err := os.ReadFile("release_manifest.yaml")
	if err != nil {
		return nil, err
	}

	manifest := new(ReleaseManifest)
	if err := yaml.Unmarshal(manifestFile, manifest); err != nil {
		return nil, err
	}

	localManifestFile, err := os.ReadFile(".release_manifest.yaml")
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	localManifest := new(ReleaseManifest)
	if err := yaml.Unmarshal(localManifestFile, localManifest); err != nil {
		return nil, err
	}

	if len(manifest.Releases) > len(localManifest.Releases) {
		err := os.WriteFile(".release_manifest.yaml", manifestFile, 0644)
		if err != nil {
			return nil, err
		}

		return manifest, nil
	}

	return localManifest, nil
}

func WriteReleaseManifest(manifest *ReleaseManifest, file string) error {
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}

	return os.WriteFile(file, manifestData, 0644)
}

type InitCmd struct{}

func (cmd *InitCmd) Run(ctx *Context) error {
	system, err := loadSystem()
	if err != nil {
		return err
	}

	if err := applyLabelsTaints(system, ctx); err != nil {
		return err
	}

	if err := installCertManager(ctx); err != nil {
		return err
	}

	if err := installMPIOperator(ctx); err != nil {
		return err
	}

	return nil
}

func applyLabelsTaints(system *config.System, ctx *Context) error {
	// Labels/Taints to apply to manager nodes and nnf nodes
	managerLabels := []string{
		"cray.nnf.manager=true",
	}
	nnfNodeLabels := []string{
		"cray.nnf.node=true",
	}
	nnfNodeTaints := []string{
		"cray.nnf.node=true:NoSchedule",
	}

	// By default, managers are workers; nodes are rabbits
	managers := system.Workers
	nnfNodes := []string{}
	for rabbit := range system.Rabbits {
		nnfNodes = append(nnfNodes, rabbit)
	}

	fmt.Printf("Applying manager labels to worker nodes: %s...\n", strings.Join(managers, ", "))
	for _, node := range managers {
		if err := runKubectlLabelOrTaint(ctx, node, "label", managerLabels); err != nil {
			return err
		}
	}

	fmt.Printf("Applying NNF node labels and taints to rabbit nodes: %s...\n", strings.Join(nnfNodes, ", "))
	for _, node := range nnfNodes {
		if err := runKubectlLabelOrTaint(ctx, node, "label", nnfNodeLabels); err != nil {
			return err
		}

		if err := runKubectlLabelOrTaint(ctx, node, "taint", nnfNodeTaints); err != nil {
			return err
		}
	}

	return nil
}

func runKubectlLabelOrTaint(ctx *Context, node string, kctlCmd string, labelsOrTaints []string) error {
	args := []string{kctlCmd, "--overwrite=true", "node", node}
	args = append(args, labelsOrTaints...)
	cmd := exec.Command("kubectl", args...)
	if _, err := runCommand(ctx, cmd); err != nil {
		return err
	}
	return nil
}

func installCertManager(ctx *Context) error {
	fmt.Println("Installing cert manager...")
	cmd := exec.Command("bash", "-c", "source common.sh; install_cert_manager")
	if _, err := runCommand(ctx, cmd); err != nil {
		return err
	}

	return nil
}

func installMPIOperator(ctx *Context) error {
	fmt.Println("Installing mpi-operator...")
	cmd := exec.Command("bash", "-c", "source common.sh; install_mpi_operator")
	if _, err := runCommand(ctx, cmd); err != nil {
		return err
	}

	return nil
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

			return "", fmt.Errorf("Cluster Name '%s' not found", context.Context.Cluster)
		}
	}

	return "", fmt.Errorf("Current Context '%s' not found", currentContext)
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
	src, err := runCommand(ctx, exec.Command("md5sum", name))
	if err != nil {
		return false, err
	}
	fmt.Printf("%s", src)

	fmt.Printf("    Destination MD5: ")
	dest, err := runCommand(ctx, exec.Command("ssh", "-q", compute, "md5sum "+path.Join(destination, name), " || true"))
	if err != nil {
		return false, err
	}
	fmt.Printf("%s", dest)

	needsUpdate := !compareMD5(src, dest)
	if needsUpdate {
		fmt.Printf("  Compute Node %s requires update to %s\n", compute, name)
	}

	if ctx.DryRun {
		needsUpdate = false
		fmt.Printf("  Dry-Run: Skipping update of '%s'\n", name)
	}

	return needsUpdate, nil
}

func copyToNode(ctx *Context, name string, compute string, destination string) error {
	fmt.Printf("  Copying %s to %s at %s...", name, compute, destination)
	if _, err := runCommand(ctx, exec.Command("scp", "-C", name, compute+":"+destination)); err != nil {
		return err
	}

	fmt.Printf("\n")
	return nil
}

func currentContext() (string, error) {
	out, err := exec.Command("kubectl", "config", "current-context").Output()
	return strings.TrimRight(string(out), "\r\n"), err
}

func loadSystem() (*config.System, error) {
	fmt.Println("Retrieving Context...")
	ctx, err := currentContext()
	if err != nil {
		return nil, err
	}

	fmt.Println("Retrieving System Config...")
	system, err := config.FindSystem(ctx)
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
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	return strings.TrimRight(string(out), "\r\n"), err
}

func repoURL() (string, error) {
	out, err := exec.Command("git", "config", "--get", "remote.origin.url").Output()
	return strings.TrimRight(string(out), "\r\n"), err
}

func checkoutCommit(commit string) error {
	return exec.Command("git", "checkout", commit).Run()
}

func currentTag() (string, error) {
	out, err := exec.Command("git", "tag", "--points-at", "HEAD").Output()
	return strings.TrimRight(string(out), "\r\n"), err
}

func addTag(tag string) error {
	return exec.Command("git", "tag", "-a", tag, "-m", fmt.Sprintf("\"setting version %s\"", tag)).Run()
}

func getOverlay(system *config.System, module string) (string, error) {

	repo, err := config.FindRepository(module)
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

func artifactoryVersion(url, commit string) (string, error) {
	out, err := exec.Command("curl", url).Output()
	if err != nil {
		return "", err
	}

	// Artifactory will return a laundry list of hrefs; try and locate the one with the right commit message
	scanner := bufio.NewScanner(bytes.NewBuffer(out))
	for scanner.Scan() {
		text := scanner.Text()
		if strings.Contains(text, commit) {
			start := strings.Index(text, "<a href=\"")
			end := strings.Index(text, "\">")
			return text[start+len("<a href=\"") : end-len("\">")+1], nil
		}
	}

	return "", fmt.Errorf("Commit %s Not Found", commit)
}

func deployModule(ctx *Context, system *config.System, module string) error {

	cmd := exec.Command("make", "deploy")

	overlay, err := getOverlay(system, module)
	if err != nil {
		return err
	}

	if system.Name == "kind" {
		// TODO: Do a sanity check to make sure the image is present on the kind nodes. This ensures
		//       that a "kind-push" was done. This would _not_ guarantee that a docker-build was done with
		//       the latest code but at least we wouldn't get an ImagePullFailure. One can get the list of
		//       images present on a cluster node by using `docker exec -it [NODE NAME] crictl images`

		if len(overlay) != 0 {
			cmd.Env = append(os.Environ(),
				"OVERLAY="+overlay)
		}

	} else {

		fmt.Print("  Finding Repository...")
		repo, err := config.FindRepository(module)
		if err != nil {
			return err
		}
		fmt.Printf(" %s\n", repo.Name)

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

		fmt.Print("  Loading From GHCR...")
		version := commit
		imageTagBase := strings.TrimSuffix(strings.TrimPrefix(url, "https://"), "/") // According to Tony; docker assumes a secure repo and prepends https when it fetches the image; so we drop it here.

		cmd.Env = append(os.Environ(),
			"IMAGE_TAG_BASE="+imageTagBase,
			"VERSION="+version,
			"OVERLAY="+overlay,
		)
	}

	fmt.Println("  Running Deploy...")
	_, err = runCommand(ctx, cmd)
	return err
}

func runCommand(ctx *Context, cmd *exec.Cmd) ([]byte, error) {
	if ctx.DryRun {
		fmt.Printf("  Dry-Run: Skipping command '%s'\n", cmd.String())
		return nil, nil
	}

	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("%s\n", stdoutStderr)

		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			fmt.Printf("Exit Error: %s (%d)\n", exitErr, exitErr.ExitCode())
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

func shouldSkipModule(module string, permittedModulesOrEmpty []string) bool {
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

func deleteSystemConfigFromSOS(ctx *Context, system *config.System, module string) error {
	if !strings.Contains(module, "nnf-sos") {
		return nil
	}

	// Check if the SystemConfiguration resource exists, and return if it doesn't
	getCmd := exec.Command("kubectl", "get", "systemconfiguration", "default", "--no-headers")
	if _, err := runCommand(ctx, getCmd); err != nil {
		return nil
	}

	fmt.Println("Deleting SystemConfiguration")
	deleteCmd := exec.Command("kubectl", "delete", "systemconfiguration", "default")

	if _, err := runCommand(ctx, deleteCmd); err != nil {
		return err
	}

	// Wait until the SystemConfiguration resource is completely gone. This may take
	// some time if there are many compute node namespaces to delete
	for true {
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

	fmt.Println("Creating SystemConfiguration...")

	config := dwsv1alpha1.SystemConfiguration{}

	config.Name = "default"
	config.Namespace = "default"
	config.Kind = "SystemConfiguration"
	config.APIVersion = fmt.Sprintf("%s/%s", dwsv1alpha1.GroupVersion.Group, dwsv1alpha1.GroupVersion.Version)

	for storageName, computes := range system.Rabbits {
		storage := dwsv1alpha1.SystemConfigurationStorageNode{}
		storage.Type = "Rabbit"
		storage.Name = storageName
		for index, computeName := range computes {
			compute := dwsv1alpha1.SystemConfigurationComputeNode{
				Name: computeName,
			}
			config.Spec.ComputeNodes = append(config.Spec.ComputeNodes, compute)

			computeReference := dwsv1alpha1.SystemConfigurationComputeNodeReference{
				Name:  computeName,
				Index: index,
			}
			storage.ComputesAccess = append(storage.ComputesAccess, computeReference)
		}
		config.Spec.StorageNodes = append(config.Spec.StorageNodes, storage)
	}

	configjson, err := json.Marshal(config)
	if err != nil {
		return err
	}

	cmd := exec.Command("bash", "-c", fmt.Sprintf("cat <<EOF | kubectl apply -f - \n%s", configjson))
	_, err = runCommand(ctx, cmd)
	return err
}
