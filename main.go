package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/alecthomas/kong"
	"gopkg.in/yaml.v2"

	"github.hpe.com/hpe/hpc-rabsw-nnf-deploy/config"
)

// This is the order in which we process the modules on deployment.
var modules = []string{
	"hpc-dpm-dws-operator",
	"hpc-rabsw-lustre-csi-driver",
	"hpc-rabsw-lustre-fs-operator",
	"hpc-rabsw-nnf-sos",
	"hpc-rabsw-nnf-dm",
}

type Context struct {
	Debug  bool
	DryRun bool
}

var cli struct {
	Debug  bool `help:"Enable debug mode."`
	DryRun bool `help:"Show what would be run."`

	Deploy   DeployCmd   `cmd:"" help:"Deploy to current context."`
	Undeploy UndeployCmd `cmd:"" help:"Undeploy from current context."`
	Make     MakeCmd     `cmd:"" help:"Run make in ever repository."`
	Install  InstallCmd  `cmd:"" help:"Install daemons."`
}

func main() {
	ctx := kong.Parse(&cli)
	err := ctx.Run(&Context{Debug: cli.Debug, DryRun: cli.DryRun})
	if err != nil {
		fmt.Printf("%v", err)
	}
	ctx.FatalIfErrorf(err)
}

type DeployCmd struct{}

func (*DeployCmd) Run(ctx *Context) error {
	system, err := loadSystem()
	if err != nil {
		return err
	}

	return runInModules(modules, func(module string) error {
		fmt.Printf("Deploying Module %s...\n", module)
		if err := deployModule(ctx, system, module); err != nil {
			return err
		}

		return nil
	})
}

type UndeployCmd struct{}

func (*UndeployCmd) Run(ctx *Context) error {
	system, err := loadSystem()
	if err != nil {
		return err
	}

	reversed := make([]string, len(modules))
	for i := range modules {
		reversed[i] = modules[len(modules)-i-1]
	}

	return runInModules(reversed, func(module string) error {
		fmt.Printf("Undeploying Module %s...\n", module)

		overlay, err := getOverlay(system, module)
		if err != nil {
			return err
		}

		cmd := exec.Command("make", "undeploy")

		if len(overlay) != 0 {
			cmd.Env = append(os.Environ(),
				"OVERLAY="+overlay,
			)
		}

		fmt.Println("  Running Undeploy...")
		if ctx.DryRun == false {
			if err := cmd.Run(); err != nil {
				return err
			}
		}

		return nil
	})
}

type MakeCmd struct {
	Command string `arg:"" name:"command" help:"Make target."`
}

func (cmd *MakeCmd) Run(ctx *Context) error {
	system, err := loadSystem()
	if err != nil {
		return err
	}

	return runInModules(modules, func(module string) error {

		fmt.Printf("Running Make %s in %s...\n", cmd.Command, module)

		cmd := exec.Command("make", cmd.Command)

		overlay, err := getOverlay(system, module)
		if err != nil {
			return err
		}

		if len(overlay) != 0 {
			cmd.Env = append(os.Environ(),
				"OVERLAY="+overlay,
			)
		}

		if ctx.DryRun == false {
			err = cmd.Run()
		}

		return nil
	})
}

type InstallCmd struct{}

func (*InstallCmd) Run(ctx *Context) error {

	system, err := loadSystem()
	if err != nil {
		return err
	}

	clusterConfig, err := currentClusterConfig()
	if err != nil {
		return err
	}

	clusterConfig = strings.TrimPrefix(clusterConfig, "https://")

	k8sServerHost := clusterConfig[:strings.Index(clusterConfig, ":")]
	k8sServerPort := clusterConfig[strings.Index(clusterConfig, ":")+1:]

	return config.EnumerateDaemons(func(d config.Daemon) error {

		var token []byte
		var cert []byte
		if d.ServiceAccount.Name != "" {
			fmt.Printf("Loading Service Account Cert & Token\n")

			fmt.Print("  Secret...")
			secret, err := exec.Command("bash", "-c", fmt.Sprintf("kubectl get serviceaccount %s -n %s -o json | jq -Mr '.secrets[].name | select(contains(\"token\"))'", d.ServiceAccount.Name, d.ServiceAccount.Namespace)).Output()
			if err != nil {
				return err
			}
			secret = secret[:len(secret)-1]
			fmt.Printf("Loaded %s\n", secret)

			fmt.Printf("  Token...")
			token, err = exec.Command("bash", "-c", fmt.Sprintf("kubectl get secret %s -n %s -o json | jq -Mr '.data.token' | base64 -D", string(secret), d.ServiceAccount.Namespace)).Output()
			if err != nil {
				return err
			}
			fmt.Printf("Loaded REDACTED\n")

			fmt.Printf("  Cert...")
			cert, err = exec.Command("bash", "-c", fmt.Sprintf("kubectl get secret %s -n %s -o json | jq -Mr '.data[\"ca.crt\"]' | base64 -D", string(secret), d.ServiceAccount.Namespace)).Output()
			if err != nil {
				return err
			}
			fmt.Printf("Loaded REDACTED\n")
		}

		err = runInModules([]string{d.Repository}, func(module string) error {

			if err := os.Chdir(d.Path); err != nil {
				return err
			}

			cmd := exec.Command("go", "build", "-o", d.Bin)
			cmd.Env = append(os.Environ(),
				"CGO_ENABLED=0",
				"GOOS=linux",
				"GOARCH=amd64",
				"GOPRIVATE=github.hpe.com",
			)

			if ctx.DryRun == false {
				if err := cmd.Run(); err != nil {
					return err
				}
			}

			for rabbit := range system.Rabbits {
				for _, compute := range system.Rabbits[rabbit] {
					fmt.Printf("Installing %s on Compute %s\n", d.Name, compute)

					if err := copyToNode(d.Bin, compute, "/usr/bin"); err != nil {
						return err
					}

					fmt.Println("  Installing service...")
					if err := exec.Command("ssh", compute, "/usr/bin/"+d.Bin, "install", "||", "true").Run(); err != nil {
						return err
					}

					configDir := "/etc/"+d.Bin
					if len(token) != 0 || len(cert) != 0 {
						if err := exec.Command("ssh", compute, "mkdir -p "+configDir).Run(); err != nil {
							return err
						}
					}

					serviceTokenPath := configDir
					if len(token) != 0 {
						if err := os.WriteFile("service.token", token, os.ModePerm); err != nil {
							return err
						}

						err = copyToNode("service.token", compute, serviceTokenPath)
						os.Remove("service.token")

						if err != nil {
							return err
						}
					}

					certFilePath := configDir
					if len(cert) != 0 {
						if err := os.WriteFile("service.cert", cert, os.ModePerm); err != nil {
							return err
						}

						err = copyToNode("service.cert", compute, certFilePath)
						os.Remove("service.cert")

						if err != nil {
							return err
						}
					}

					execStart := ""
					execStart += "[Service]\n"
					execStart += "ExecStart=\n"
					execStart += "ExecStart=/usr/bin/" + d.Bin + " \\\n"
					execStart += "  --kubernetes-service-host=" + k8sServerHost + " \\\n"
					execStart += "  --kubernetes-service-port=" + k8sServerPort + " \\\n"
					execStart += "  --node-name=" + compute + " \\\n"
					execStart += "  --nnf-node-name=" + rabbit + " \\\n"
					if len(token) != 0 {
						execStart += "  --" + strings.Replace(d.Name, "_", "-", -1) + "-service-token-file=" + path.Join(serviceTokenPath, "service.token") + " \\\n"
					}
					if len(cert) != 0 {
						execStart += "  --" + strings.Replace(d.Name, "_", "-", -1) + "-service-cert-file=" + path.Join(certFilePath, "service.cert") + " \\\n"
					}

					fmt.Println("  Creating override directory...")
					overridePath := "/etc/systemd/system/" + d.Bin + ".service.d"
					if err := exec.Command("ssh", compute, "mkdir", "-p", overridePath).Run(); err != nil {
						return err
					}

					fmt.Println("  Creating override configuration...")
					if err := os.WriteFile("override.conf", []byte(execStart), os.ModePerm); err != nil {
						return err
					}

					err = copyToNode("override.conf", compute, overridePath)
					os.Remove("override.conf")

					if err != nil {
						return err
					}

					// Reload the daemon to pick up the override.conf.
					fmt.Println("  Reloading service...")
					if err := exec.Command("ssh", compute, "systemctl daemon-reload").Run(); err != nil {
						return err
					}

					fmt.Println("  Starting service...")
					if err := exec.Command("ssh", compute, "systemctl start "+d.Bin).Run(); err != nil {
						return err
					}
				}
			}

			return nil
		})

		return err
	})
}

type k8scluster struct {
	Name    string
	Cluster struct {
		Server string
	}
}
type k8sConfig struct {
	Kind     string
	Clusters []k8scluster
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

	for _, cluster := range config.Clusters {
		if cluster.Name == string(current) {
			return cluster.Cluster.Server, nil
		}
	}

	return "", fmt.Errorf("Current Cluster %s not found", current)
}

func copyToNode(name string, compute string, destination string) error {
	fmt.Printf("  Copying %s to %s at %s\n", name, compute, destination)
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
	src, err := exec.Command("md5sum", name).Output()
	if err != nil {
		return err
	}
	fmt.Printf("%s", src)

	fmt.Printf("    Destination MD5: ")
	dest, err := exec.Command("ssh", compute, "md5sum "+path.Join(destination, name)+" || true").Output()
	if err != nil {
		return err
	}
	fmt.Printf("%s", dest)

	if !compareMD5(src, dest) {
		fmt.Printf("    Copying...")
		if err := exec.Command("scp", name, compute+":"+destination).Run(); err != nil {
			return err
		}
	}

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
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	return strings.TrimRight(string(out), "\r\n"), err
}

func getOverlay(system *config.System, module string) (string, error) {

	fmt.Println("  Finding Overlay...")
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

	if overlay != "kind" {

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

		fmt.Print("  Loading From Artifactory...")
		version, err := artifactoryVersion(url, commit)
		if err != nil {
			return err
		}
		fmt.Printf(" %s\n", version)

		imageTagBase := strings.TrimSuffix(strings.TrimPrefix(url, "https://"), "/") // According to Tony; docker assumes a secure repo and prepends https when it fetches the image; so we drop it here.
		imageTagBase = strings.Replace(imageTagBase, "/artifactory", "", 1)

		cmd.Env = append(os.Environ(),
			"IMAGE_TAG_BASE="+imageTagBase,
			"VERSION="+version,
			"OVERLAY="+overlay,
		)
	}

	fmt.Println("  Running Deploy...")
	if ctx.DryRun == false {
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
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
