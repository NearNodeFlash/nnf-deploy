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

set -e
set -o pipefail

if kubectl get nnfstorageprofile -n nnf-system default > /dev/null 2>&1; then
    exit 0
fi

TMPIN=$(mktemp)

if ! kubectl get nnfstorageprofile -n nnf-system template -o json > "$TMPIN";  then
    echo "Unable to retrieve NnfStorageProfile/template"
    rm -f "$TMPIN"
    exit 1
fi

TMPOUT=$(mktemp)

if ! jq '.data.default |= true | del(.metadata) | .metadata |= {name: "default", namespace: "nnf-system"}' "$TMPIN" > "$TMPOUT"; then
    echo "Unable to build new default NnfStorageProfile manifest"
    rm -f "$TMPIN" "$TMPOUT"
    exit 1
fi

if ! kubectl apply --server-side=true -f "$TMPOUT"; then
    echo "Unable to create NnfStorageProfile/default"
    rm -f "$TMPIN" "$TMPOUT"
    exit 1
fi

rm -f "$TMPIN" "$TMPOUT"
exit 0
