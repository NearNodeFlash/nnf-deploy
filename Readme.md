# NNF Deployment

To clone this project, use the additional --recurse-submodules option to retrieve its submodules:
```
git clone --recurse-submodules git@github.hpe.com:hpe/hpc-rabsw-nnf-deploy
```

Build using `go build`

Prior to running, ensure correct NNF systems are loaded in [./config/systems.yaml](./config/systems.yaml) and correct Artifactory repositories are defined in [./config/artifactory.yaml](./config/artifactory.yaml)

## Deploying
Deploying will deploy all the submodules to your current kube config context

`hpc-rabsw-nnf-deploy deploy`

## Undeploying
Undeploy all the submodules

`hpc-rabsw-nnf-deploy undeploy`