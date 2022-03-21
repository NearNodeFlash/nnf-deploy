#!/bin/bash

# Build in each submodule with the submodule defaults. If you wish to use an overlay
# see "hpc-rabsw-nnf-deploy make docker-build"; this will use the overlays defined
# in the ./config/systems.yaml

source common.sh

for SUBMODULE in $SUBMODULES; do
    ( cd $SUBMODULE && make docker-build )
done