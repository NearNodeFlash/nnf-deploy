****# NNF Deployment

To clone this project, use the additional --recurse-submodules option to retrieve its submodules:

```bash
git clone --recurse-submodules git@github.com:NearNodeFlash/nnf-deploy
```

## Updating the Submodules

To update the submodules in this work area, run the [update.sh](./update.sh) script.  Use this to pick up recent changes in any of the submodules.

**Warning** If the current submodules have already been deployed to a K8s system, then teardown and delete any workflows and run `nnf-deploy undeploy` to remove the old CRDs and pods prior to updating the submodules.  An update may pull in new CRD changes that are incompatible with resources that are already on the K8s system.

The update.sh command will update each submodule directory to the head of its master branch.

```bash
./update.sh
```

### Submodule Versions

Any submodule can be set to a specific revision and it will be used by the nnf-deploy command.  Note the warning above prior to setting a submodule to a specific revision.

To set a submodule to a specific revision, change into that submodule's directory and switch to that revision or branch:

```bash
cd nnf-sos
git switch branch-with-my-fixes
cd ..
```

The update.sh command will switch that submodule back to the head of its master branch.

## nnf-deploy

nnf-deploy is a golang executable capable of building components of the Rabbit software stack locally as well as deploying and un-deploying those components to a k8s cluster specified by the current kubeconfig.

### Build

Build using: `go build`

Prior to running, ensure correct NNF systems are loaded in [./config/systems.yaml](./config/systems.yaml) and correct ghcr repositories are defined in [./config/repositories.yaml](./config/repositories.yaml)

### Options

```bash
./nnf-deploy --help
Usage: nnf-deploy <command>

Flags:
  -h, --help       Show context-sensitive help.
      --debug      Enable debug mode.
      --dry-run    Show what would be run.

Commands:
  init
    Initialize cluster.

  deploy
    Deploy to current context.

  undeploy
    Undeploy from current context.

  make <command>
    Run make [COMMAND] in every repository.

  install
    Install daemons.

Run "nnf-deploy <command> --help" for more information on a command.
```

## Init

The `init` subcommand applies the proper labels and taints to the cluster nodes. It also installs
cert manager via `common.sh`. This only needs to be done once on a new cluster.

Note: This behavior replaces the `init`.sh script, which has been removed.

The manager nodes (worker nodes) will obtain the following labels:

- `cray.nnf.manager=true`

Additionally, the NNF nodes (rabbit nodes) will obtain the `"cray.nnf.node=true"`
label and the `"cray.nnf.node=true:NoSchedule"` taint.

These labels/taint will be applied using the `--overwrite=true` option to `kubectl`.

Once the labels/taint are applied, cert manager will be installed.

```bash
./nnf-deploy init
```

## Deploy

Deploy all the submodules using the `deploy` command

```bash
./nnf-deploy deploy
```

To deploy only specific repositories, include the desired modules after `deploy` command. For example, to deploy only `dws` and `nnf-sos` repositories, use
```bash
./nnf-deploy deploy dws nnf-sos
```

## Undeploy

**WARNING!** Before you undeploy, delete any user or administrator created resources such as `lustrefilesystems` and `workflows` using kubectl commands

```bash
kubectl delete workflows.dws.cray.hpe.com --all
kubectl delete lustrefilesystems.cray.hpe.com --all
```

Undeploy all the submodules using the `undeploy` command.

```bash
./nnf-deploy undeploy
```

Similar to deploy, you may undeploy specific repositories by including the desired modules after the `undeploy` command. For example, to undeploy only `dws` and `nnf-sos`, use

```bash
./nnf-deploy undeploy dws nnf-sos
```

## Make

The `make` subcommand provides direct access to makefile targets within each submodule in nnf-deploy executing `make <command>` within each submodule. For example, the following command performs a `docker-build` within each submodule:

```bash
./nnf-deploy make docker-build
```

### Kind cluster

Kind clusters are built and deployed using locally compiled images. The following commands:

- Create a kind cluster
- Build all docker images for Rabbit modules
- Push those images into the Kind cluster
- Deploy those images onto the Kind cluster nodes

```bash
./kind.sh reset
./nnf-deploy make docker-build
./nnf-deploy make kind-push
./nnf-deploy deploy
```

## Install

The `install` subcommand will compile and install the daemons on the compute nodes, along with the
proper certs and tokens. Systemd files are used to manage and start the daemons. This is necessary for data movement.

```bash
./nnf-deploy install
```

# Testing

NNF test infrastructure and individualized tests reside in the [/test](./test/) directory. Tests are expected to run against a fully deployed cluster reachable via your current k8s configuration context. NNF test uses the [Ginkgo](https://onsi.github.io/ginkgo) test framework.

Various Ginkgo options can be passed into `go test`. Common options include `-ginkgo.fail-fast`,  `-ginkgo.progress`,  and `-ginkgo.v`

```bash
go test -v ./test/... -ginkgo.fail-fast -ginkgo.progress -ginkgo.v
```

Ginkgo also provides the [Ginkgo CLI](https://onsi.github.io/ginkgo/#ginkgo-cli-overview) that can be used for enhanced test features like parallelization, randomization, and filter.

## Test Definitions

Individual tests are listed in [/test/int_test.go](./test/int_test.go). Tests are written from the perspective of a workload manager and should operate only on DWS resources when possible.

## Test Options

[Test Options](./test/internal/options.go) allow the user extend test definitions with various options. Administrative controls, like creating NNF Storage Profiles or NNF Container profiles, configuring a global Lustre File System, or extracting Lustre parameters from a persistent Lustre instance, are some example test options.
