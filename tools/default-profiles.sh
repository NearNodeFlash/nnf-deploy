#!/bin/bash

# Copyright 2024-2025 Hewlett Packard Enterprise Development LP
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

set -e
set -o pipefail


mk_default() {
    local crd="$1"
    local profnamespace="$2"
    local proftemplate="$3"
    local profdefault="$4"
    local TMPIN
    local TMPOUT
    local profkind

    if ! kubectl get crd "$crd" 2>/dev/null 1>&2; then
        return
    fi

    profkind=$(echo "$crd" | awk -F. '{print $1}')
    if kubectl get "$profkind" -n "$profnamespace" "$profdefault" > /dev/null 2>&1; then
        return
    fi

    TMPIN=$(mktemp)

    if ! kubectl get "$profkind" -n "$profnamespace" "$proftemplate" -o json > "$TMPIN" 2>&1;  then
        if [[ $(<"$TMPIN") =~ "not found" ]]; then
            return
        fi
        echo "Unable to retrieve $profkind/$proftemplate: $(<"$TMPIN")"
        rm -f "$TMPIN"
        exit 1
    fi

    TMPOUT=$(mktemp)

    if [[ "$profdefault" == "default" ]]; then
        # If its name will be "default" and it has a .data.default, then set
        # that to true.
        if ! jq .data.default "$TMPIN" | grep -q null; then
            if ! jq ".data.default |= true" "$TMPIN" > "$TMPOUT"; then
                echo "Unable to set default in new $profkind/$profdefault manifest"
                rm -f "$TMPIN" "$TMPOUT"
                exit 1
            fi
            mv "$TMPOUT" "$TMPIN"
        fi
    fi
    if ! jq "del(.metadata) | .metadata |= {name: \"$profdefault\", namespace: \"$profnamespace\"}" "$TMPIN" > "$TMPOUT"; then
        echo "Unable to build new $profkind/$profdefault manifest"
        rm -f "$TMPIN" "$TMPOUT"
        exit 1
    fi

    if ! kubectl apply --server-side=true -f "$TMPOUT"; then
        echo "Unable to create $profkind/$profdefault"
        rm -f "$TMPIN" "$TMPOUT"
        exit 1
    fi

    rm -f "$TMPIN" "$TMPOUT"
}

mk_default "nnfstorageprofiles.nnf.cray.hpe.com" "nnf-system" "template" "default"
mk_default "nnfcontainerprofiles.nnf.cray.hpe.com" "nnf-system" "copy-offload-template" "copy-offload-default"
mk_default "nnfdatamovementprofiles.nnf.cray.hpe.com" "nnf-system" "copy-offload-template" "copy-offload-default"

exit 0
