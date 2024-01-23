#!/bin/bash

# Copyright 2024 Hewlett Packard Enterprise Development LP
# Other additional copyright holders may be indicated within.
#
# The entirety of this work is licensed under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
#
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

TREEDIR="$1"

if [[ -z $TREEDIR ]]; then
    echo "specify a tree dir"
    exit 1
elif [[ $TREEDIR != /* ]]; then
    echo "must begin with a slash"
    exit 1
elif [[ -d $TREEDIR ]]; then
    echo "the dir already exists"
    exit 1
fi

mkdir "$TREEDIR"
DO_MODULES=""
SUBMODULES=$(git submodule status | awk '{print $2}')

for SUBMODULE in $SUBMODULES; do
    if grep -qE '^edit-image:' "$SUBMODULE"/Makefile
    then
        echo "Generating the manifest from $SUBMODULE"
        if ! make -C "$SUBMODULE" edit-image kustomize; then
            echo "Stopping at $SUBMODULE"
            exit 1
        fi
        DO_MODULES="$DO_MODULES $SUBMODULE"
    fi
done

for SUBMODULE in $DO_MODULES; do
    echo "Collecting the manifest from $SUBMODULE"
    mkdir "$TREEDIR/$SUBMODULE"
    (cd "$SUBMODULE" || exit 1
     if [[ -d config/begin ]]; then
         bin/kustomize build config/begin > "$TREEDIR/$SUBMODULE/$SUBMODULE.yaml"
     fi
     if [[ -d config/begin-examples ]]; then
         bin/kustomize build config/begin-examples > "$TREEDIR/$SUBMODULE/$SUBMODULE-examples.yaml"
     fi
     if [[ -d config/prometheus ]]; then
         bin/kustomize build config/prometheus > "$TREEDIR/$SUBMODULE/$SUBMODULE-prometheus.yaml"
     fi
     if [[ -d deploy/kubernetes/begin ]]; then
         bin/kustomize build deploy/kubernetes/begin > "$TREEDIR/$SUBMODULE/$SUBMODULE.yaml"
     fi
    )
done

mkdir "$TREEDIR/cert-mgr"
CERT_URL=$(yq -M '.thirdPartyServices[] | select(.name == "cert-manager") | .url' config/repositories.yaml)
wget -O "$TREEDIR"/cert-mgr/cert-mgr.yaml "$CERT_URL"

mkdir "$TREEDIR/mpi-operator"
MPIOP_URL=$(yq -M '.thirdPartyServices[] | select(.name == "mpi-operator") | .url' config/repositories.yaml)
wget -O "$TREEDIR"/mpi-operator/mpi-operator.yaml "$MPIOP_URL"


