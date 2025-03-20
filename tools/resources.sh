#!/bin/bash

# Copyright 2023 Hewlett Packard Enterprise Development LP
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

usage() {
    cat <<EOF
Run various debug commands against an NNF cluster

Usage: $0 COMMAND [ARGS...]

Commands:
    get-all [OUTPUT-FORMAT]             Run 'kubectl get ...' against all DWS and NNF related resources
                                        with the desired OUTPUT-FORMAT (default: yaml)

    remove-finalizers                   Remove the finalizers in all DWS and NNF related resources.
                                        WARNING: Can result in possible system corruption. Use at your
                                        own risk.
EOF
}

if [ $# -lt 1 ]; then
    usage
    exit 1
fi

# for_all_resources <func CRD NAME NAMESPACE ARGS...>
for_all_resources() {
    local FUNCTION=$1 ARGS=( "${@:2}" )

    CRDS=$(kubectl get crds | grep -E "(nnf|lustrefilesystems)\.(cray\.)+hpe\.com|dataworkflowservices.github.io" | awk '{print $1}')
    for CRD in $CRDS
    do
        echo "Processing CRD $CRD"
        IFS=$'\n' RESOURCES=$(kubectl get $CRD --all-namespaces --no-headers)
        for RESOURCE in $RESOURCES
        do
            NAMESPACE=$(echo $RESOURCE | awk '{print $1}')
            NAME=$(echo $RESOURCE | awk '{print $2}')

            echo "  Resource $NAMESPACE/$NAME"
            "$FUNCTION" "$CRD" "$NAME" "$NAMESPACE" "${ARGS[@]}"
        done
    done
}

case $1 in
    get-all)
        function get {
            local CRD=$1 NAME=$2 NAMESPACE=$3 OUTPUT=$4

            kubectl get "$CRD"/"$NAME" --namespace "$NAMESPACE" --output "$OUTPUT"
        }

        for_all_resources get "${2:-yaml}"
        ;;
    remove-finalizers)
        function remove_finalizers() {
            local CRD=$1 NAME=$2 NAMESPACE=$3

            kubectl patch "$CRD"/"$NAME" --namespace "$NAMESPACE" --type merge --patch '{"metadata": {"finalizers": []}}'
        }

        for_all_resources remove_finalizers
        ;;
    *)
        usage
        exit 1
        ;;
esac