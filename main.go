package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/kong"

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

	Make MakeCmd `cmd:"" help:"Run make in ever repository."`
}

func main() {
	ctx := kong.Parse(&cli)
	err := ctx.Run(&Context{Debug: cli.Debug, DryRun: cli.DryRun})
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
