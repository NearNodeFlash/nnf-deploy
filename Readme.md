# NNF Deployment

To clone this project, use the additional --recurse-submodules option to retrieve its submodules:

```bash
git clone --recurse-submodules git@github.hpe.com:hpe/hpc-rabsw-nnf-deploy
```

## hpc-rabsw-nnf-deploy

hpc-rabsw-nnf-deploy is a golang executable capable of building all of the docker components of the Rabbit software stack locally as well as deploying and undeploying those components to a k8s cluster specified by the current kube config.

### Build

Build using: `go build`

Prior to running, ensure correct NNF systems are loaded in [./config/systems.yaml](./config/systems.yaml) and correct Artifactory repositories are defined in [./config/artifactory.yaml](./config/artifactory.yaml)

### Options

```bash
./hpc-rabsw-nnf-deploy --help
Usage: hpc-rabsw-nnf-deploy <command>

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

Run "hpc-rabsw-nnf-deploy <command> --help" for more information on a command.

```

## Deploy

Deploying will deploy all the submodules to your current kube config context

```bash
./hpc-rabsw-nnf-deploy deploy
```

## Undeploy

Undeploy all the submodules

```bash
./hpc-rabsw-nnf-deploy undeploy
```

## Make

The `make` subcommand provides direct access to makefile targets within each submodule in hpc-rabsw-nnf-deploy executing `make <command>` within each submodule. For example, the following command performs a `docker-build` within each submodule:

```bash
./hpc-rabsw-nnf-deploy make docker-build
```

### Kind cluster

Kind clusters are built and deployed using locally compiled images. The following commands:

- Create a kind cluster
- Build all docker images for Rabbit modules
- Push those images into the Kind cluster
- Deploy those images onto the Kind cluster nodes

```bash
./kind.sh reset
./hpc-rabsw-nnf-deploy make docker-build
./hpc-rabsw-nnf-deploy make kind-push
./hpc-rabsw-nnf-deploy make deploy
```

## Install
<TBD>