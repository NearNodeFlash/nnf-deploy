package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

var (
	dryrun bool
)

func usage() {
	fmt.Println("Near Node Flash (NNF) Deployment Tool")
	fmt.Println("hpc-rabsw-nnf-deploy [command] [options]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  deploy                deploy to current context")
	fmt.Println("  undeploy              undeploy from current context")
	os.Exit(1)
}

func main() {

	if len(os.Args) < 2 {
		usage()
		return
	}

	var err error
	switch os.Args[1] {
	case "deploy":
		deployCmd := flag.NewFlagSet("deploy", flag.ExitOnError)
		deployCmd.BoolVar(&dryrun, "dry-run", false, "dry run the deployment")
		deployCmd.Parse(os.Args[2:])
		err = deploy()
	case "undeploy":
		undeployCmd := flag.NewFlagSet("undeploy", flag.ExitOnError)
		undeployCmd.BoolVar(&dryrun, "dry-run", false, "dry run the undeployment")
		undeployCmd.Parse(os.Args[2:])
		err = undeploy()
	default:
		usage()
		return
	}

	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}
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

func deploy() error {

	system, err := loadSystem()
	if err != nil {
		return err
	}

	// Walk through repositories and run make deploy on them with the correct environmental variables
	for _, module := range modules {
		fmt.Printf("Deploying %s...\n", module)
		if err := deployModule(system, module); err != nil {
			return err
		}
	}

	return nil
}

func currentBranch() (string, error) {
	out, err := exec.Command("git", "branch", "--show-current").Output()
	return strings.TrimRight(string(out), "\r\n"), err
}

func lastLocalCommit() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	return strings.TrimRight(string(out), "\r\n"), err
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
			return text[start+len("<a href=\"") : end-len("\">")], nil
		}
	}

	return "", fmt.Errorf("Commit %s Not Found", commit)
}

func deployModule(system *config.System, module string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)

	if err := os.Chdir(module); err != nil {
		return err
	}

	fmt.Println("  Loading Current Branch...")
	branch, err := currentBranch()
	if err != nil {
		return err
	}

	fmt.Println("  Finding Repository...")
	repo, err := config.FindRepository(module)
	if err != nil {
		return err
	}

	url := repo.Master
	if branch != "master" {
		url = repo.Development
	}

	fmt.Println("  Loading Last Commit...")
	commit, err := lastLocalCommit()
	if err != nil {
		return err
	}

	fmt.Println("  Loading From Artifactory ...")
	version, err := artifactoryVersion(url, commit)
	if err != nil {
		return err
	}

	imageTagBaseEnv := strings.TrimSuffix(strings.TrimPrefix(url, "https://"), "/") // According to Tony; docker assumes a secure repo and prepends https when it fetches the image; so we drop it here.
	versionEnv := version

	fmt.Printf("  Running Deploy...")

	cmd := exec.Command("make", "deploy")
	cmd.Env = append(os.Environ(),
		"IMAGE_TAG_BASE="+imageTagBaseEnv,
		"VERSION="+versionEnv,
	)

	if dryrun == false {
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}

func undeploy() error {

	system, err := loadSystem()
	if err != nil {
		return err
	}

	for i := range modules {
		module := modules[len(modules)-i-1]
		fmt.Printf("Undeploying %s...\n", module)
		if err := undeployModule(system, module); err != nil {
			return err
		}
	}

	return nil
}

func undeployModule(system *config.System, module string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)

	if err := os.Chdir(module); err != nil {
		return err
	}

	if !dryrun {
		if err := exec.Command("make", "undeploy").Run(); err != nil {
			return err
		}
	}

	return nil
}
