# NNF Deployment

To clone this project, use the additional --recurse-submodules option to retrieve its submodules:

```bash
git clone --recurse-submodules git@github.com:NearNodeFlash/nnf-deploy
```

## Updating the Submodules

To update the submdules in this workarea, run the update.sh script in this workarea.  Use this to pick up recent changes in any of the submodules.

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

nnf-deploy is a golang executable capable of building all of the docker components of the Rabbit software stack locally as well as deploying and undeploying those components to a k8s cluster specified by the current kube config.

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
  deploy
    Deploy to current context.

  undeploy
    Undeploy from current context.

  make <command>
    Run make [COMMAND] in every repository.

  install
    Install daemons (EXPERIMENTAL).

Run "nnf-deploy <command> --help" for more information on a command.

```

## Deploy

Deploying will deploy all the submodules to your current kube config context

```bash
./nnf-deploy deploy
```

## Undeploy

Undeploy all the submodules

```bash
./nnf-deploy undeploy
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

TBD
