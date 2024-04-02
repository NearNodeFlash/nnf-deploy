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

SYSTEMS_YAML="config/systems.yaml"

while getopts 's:d:t:h' opt; do
case "$opt" in
d) TREEDIR="$OPTARG" ;;
s) SYSTEM_TYPE="$OPTARG" ;;
t) TARFILE="$OPTARG" ;;
\?|h)
    echo "Usage: $0 -s SYSTEM_TYPE -d NEW_DIR -t TARFILE_NAME"
    echo
    echo "  -d NEW_DIR       Directory to create tree of manifests. This"
    echo "                   must not exist and must begin with a slash." 
    echo "  -s SYSTEM_TYPE   Type of manifests to generate. Specify 'kind'"
    echo "                   for K8s-in-Docker or 'rabbit' for real hardware."
    echo "  -t TARFILE_NAME  Name to give to the tarfile of manifests. This"
    echo "                   must begin with a slash."
    exit 1
    ;;
esac
done
shift "$((OPTIND - 1))"

if [[ -z $TREEDIR ]]; then
    echo "You must specify -d"
    exit 1
elif [[ $TREEDIR != /* ]]; then
    echo "The directory must begin with a slash"
    exit 1
elif [[ -d $TREEDIR ]]; then
    echo "The directory $TREEDIR already exists"
    exit 1
fi
if [[ -z $TARFILE ]]; then
    echo "You must specify -t"
    exit 1
elif [[ $TARFILE != /* ]]; then
    echo "The tarfile name must begin with a slash"
    exit 1
elif [[ -f $TARFILE ]]; then
    echo "The tarfile must not already exist"
    exit 1
fi
if [[ -z $SYSTEM_TYPE ]]; then
    echo "You must specify -s"
    exit 1
elif [[ $SYSTEM_TYPE != "kind" && $SYSTEM_TYPE != "rabbit" ]]; then
    echo "System type must be 'kind' or 'rabbit'"
    exit 1
fi

if ! which python3 >/dev/null 2>&1; then
    echo "Unable to find python3 in PATH"
    exit 1
elif ! python3 -c 'import yaml' 2>/dev/null; then
    echo "Unable to find PyYAML"
    exit 1
fi

set -e
set -o pipefail

mkdir "$TREEDIR"
DO_MODULES=""
SUBMODULES=$(git submodule status | awk '{print $2}')

get_overlays() {
    local system_name="$1"

    local overlays=""
    # Wishing for yq(1)...
    if ! overlays=$(python3 -c 'import yaml, sys; doc = yaml.safe_load(sys.stdin); x = [" ".join(sys["overlays"]) for sys in doc["systems"] if sys["name"] == "'"$system_name"'"]; print(x[0])' < $SYSTEMS_YAML); then
        echo "Unable to find overlays for $system_name"
        exit 1
    fi
    echo "$overlays"
}

find_overlay_dir() {
    local subdir="$1"
    local system_name="$2"

    local overlays=""
    overlays=$(get_overlays "$system_name")
    local ovlay="NONE"
    for x in $overlays; do
        if [[ -d $subdir/config/$x || -d $subdir/deploy/kubernetes/$x ]]; then
            ovlay="$x"
            break
        fi
    done
    echo "$ovlay"
}

point_to_overlay() {
    local system_name="$1"

    for SUBMODULE in $SUBMODULES; do
        OVLAY=$(find_overlay_dir "$SUBMODULE" "$system_name")
        if grep -qE '^edit-image:' "$SUBMODULE"/Makefile
        then
            echo "Generating the manifest for overlay $OVLAY from $SUBMODULE"
            if ! make OVERLAY="$OVLAY" -C "$SUBMODULE" edit-image kustomize; then
                echo "Stopping at $SUBMODULE"
                exit 1
            fi
            DO_MODULES="$DO_MODULES $SUBMODULE"
        fi
    done
}

collect_manifest() {
    for SUBMODULE in $DO_MODULES; do
        echo "Collecting the manifest from $SUBMODULE"
        SUBMOD_DIR="$TREEDIR/$SUBMODULE"
        mkdir "$SUBMOD_DIR"
        (cd "$SUBMODULE" || exit 1
         if [[ -d config/begin ]]; then
             # Remove the namespace from the manifest, because this manifest is
             # used not only to deploy but also to undeploy.
             # The namespace will be created by the ArgoCD Application resource,
             # and nothing will delete it.
             # Place the CRDs in a separate manifest.
             # Wishing for yq(1)...
             bin/kustomize build config/begin | python3 -c 'import yaml, sys; docs=yaml.safe_load_all(sys.stdin); _ = [print("%s---" % yaml.dump(doc)) for doc in docs if doc["kind"] not in ["Namespace", "CustomResourceDefinition"]]' > "$SUBMOD_DIR/$SUBMODULE.yaml"
             bin/kustomize build config/begin | python3 -c 'import yaml, sys; docs=yaml.safe_load_all(sys.stdin); _ = [print("%s---" % yaml.dump(doc)) for doc in docs if doc["kind"] == "CustomResourceDefinition"]' > "$SUBMOD_DIR/$SUBMODULE-crds.yaml"
         fi
         if [[ -d config/begin-examples ]]; then
             bin/kustomize build config/begin-examples > "$SUBMOD_DIR/$SUBMODULE-examples.yaml"
         fi
         if [[ $SUBMODULE == nnf-sos && -d config/ports ]]; then
             bin/kustomize build config/ports > "$SUBMOD_DIR/$SUBMODULE-ports.yaml"
         fi
         if [[ -d config/prometheus ]]; then
             bin/kustomize build config/prometheus > "$SUBMOD_DIR/$SUBMODULE-prometheus.yaml"
         fi
         if [[ -d deploy/kubernetes/begin ]]; then
             # Remove the namespace from the manifest, because this manifest is
             # used not only to deploy but also to undeploy.
             # The namespace will be created by the ArgoCD Application resource,
             # and nothing will delete it.
             # Wishing for yq(1)...
             bin/kustomize build deploy/kubernetes/begin | python3 -c 'import yaml, sys; docs=yaml.safe_load_all(sys.stdin); _ = [print("%s---" % yaml.dump(doc)) for doc in docs if doc["kind"] != "Namespace"]' > "$SUBMOD_DIR/$SUBMODULE.yaml"
         fi
        )
    done
}

walk_overlays() {
    if [[ ! -f $SYSTEMS_YAML ]]; then
        echo "Unable to find $SYSTEMS_YAML"
        exit 1
    fi

    if [[ $SYSTEM_TYPE == 'kind' ]]; then
        point_to_overlay kind
        collect_manifest
    else
        point_to_overlay rabbit-tds
        collect_manifest
    fi
}

walk_overlays

mkdir "$TREEDIR/cert-mgr"
# Wishing for yq(1)...
CERT_URL=$(python3 -c 'import yaml, sys; doc = yaml.safe_load(sys.stdin); x = [tp["url"] for tp in doc["thirdPartyServices"] if tp["name"] == "cert-manager"]; print(x[0])' < config/repositories.yaml)
wget -O "$TREEDIR"/cert-mgr/cert-mgr.yaml "$CERT_URL"

mkdir "$TREEDIR/mpi-operator"
MPIOP_URL=$(python3 -c 'import yaml, sys; doc = yaml.safe_load(sys.stdin); x = [tp["url"] for tp in doc["thirdPartyServices"] if tp["name"] == "mpi-operator"]; print(x[0])' < config/repositories.yaml)
wget -O "$TREEDIR"/mpi-operator/mpi-operator.yaml "$MPIOP_URL"

(cd "$TREEDIR" && tar cf "$TARFILE" ./*)
exit $?

