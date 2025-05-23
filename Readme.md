# NNF Deployment

To clone this project, use the additional --recurse-submodules option to retrieve its submodules:

```bash
git clone --recurse-submodules git@github.com:NearNodeFlash/nnf-deploy
```

## Updating the Submodules

To update the submodules in this work area, run the [update.sh](tools/update.sh) script.  Use this to pick up recent changes in any of the submodules.

**Warning** If the current submodules have already been deployed to a K8s system, then teardown and delete any workflows and run `nnf-deploy undeploy` to remove the old CRDs and pods prior to updating the submodules.  An update may pull in new CRD changes that are incompatible with resources that are already on the K8s system.

The update.sh command will update each submodule directory to the head of its master branch.

```bash
tools/update.sh
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

Build using: `make`

Prior to running, ensure correct NNF systems are loaded in [./config/systems.yaml](./config/systems.yaml) and correct ghcr repositories are defined in [./config/repositories.yaml](./config/repositories.yaml)

### Options

```bash
./nnf-deploy --help
Usage: nnf-deploy <command>

Flags:
  -h, --help       Show context-sensitive help.
      --debug      Enable debug mode.
      --dry-run    Show what would be run.
      --systems="config/systems.yaml"
                   path to the systems config file
      --repos="config/repositories.yaml"
                   path to the repositories config file
      --daemons="config/daemons.yaml"
                   path to the daemons config file

Commands:
  deploy [<only> ...]
    Deploy to current context.

  undeploy [<only> ...]
    Undeploy from current context.

  make <command> [<only> ...]
    Run make [COMMAND] in every repository.

  install [<node> ...]
    Install daemons (EXPERIMENTAL).

  init
    Initialize cluster.

Run "nnf-deploy <command> --help" for more information on a command.
```

## Init

The `init` subcommand will install ArgoCD via helm. The user must have the helm
CLI installed. This init command should be done only once on a new cluster.

```bash
./nnf-deploy init
```

To restore legacy init behavior--to have `init` install cert manager,
mpi-operator, lustre-csi-driver, and lustre-fs-operator--copy the
`config/overlay-legacy.yaml-template` file to `./overlay-legacy.yaml`. This init
command only needs to be done once on a new cluster or when one of them
changes.

```bash
cp config/overlay-legacy.yaml-template overlay-legacy.yaml
./nnf-deploy init
```

## Deploy

Deploy all the submodules using the `deploy` command

```bash
./nnf-deploy deploy
```

To deploy only specific repositories, include the desired modules after `deploy` command. For example, to deploy only `dws` and `nnf-sos` repositories:

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

## KIND cluster

[Kubernetes-in-Docker](https://kind.sigs.k8s.io) (KIND) clusters are built and deployed using locally compiled images.

- Create a kind cluster
- Attach ArgoCD to your gitops repo, which you created from the [argocd-boilerplate](https://github.com/NearNodeFlash/argocd-boilerplate). Otherwise, see the "overlay legacy" step in "Init" above, to run the cluster without ArgoCD.
- Build all docker images for Rabbit modules
- Push those images into the Kind cluster
- Make a manifest for these images. Run `make manifests` and use the `./tools/unpack-manifest.py` tool in your gitops repo to unpack the `manifests-kind.tar` file.
- Deploy those images onto the Kind cluster nodes

```console
./tools/kind.sh create
./tools/kind.sh argocd_attach $ARGOCD_PASSWORD
./nnf-deploy make docker-build
./nnf-deploy make kind-push
```

Next, use the `./tools/deploy-env.sh` tool in your gitops repo to deploy the bootstrap resources to ArgoCD in your cluster.

### KIND with existing NNF releases

A KIND cluster may be used to test existing releases. Use the same steps described above, skipping the `nnf-deploy make ...` commands because existing releases will pull their images from the upstream registry. Download the `manifests-kind.tar` file from the desired NNF release at [nnf-deploy releases](https://github.com/NearNodeFlash/nnf-deploy/releases) and use the `./tools/unpack-manifests.py` tool in your gitops repo to unpack the manifest.

## Install

The `install` subcommand will compile and install the daemons on the compute nodes, along with the
proper certs and tokens. Systemd files are used to manage and start the daemons. This is necessary for data movement.

```bash
./nnf-deploy install
```
