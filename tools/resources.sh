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

# Dump a summary of all active resources.

kinds_dws_nnf=$(kubectl api-resources | grep -iE 'lus|dws|dataworkflowservices|nnf|dm' | awk '{print $1":"$2}' | sort)
for want_kind in dws dataworkflowservices nnf lus dm
do
    for x in $(echo "$kinds_dws_nnf" | grep ":$want_kind\." | awk -F: '{print $1}')
    do
        echo "=== $x"
        out=$(kubectl get -A --sort-by=.metadata.namespace "$x")
        if (( $(echo "$out" | wc -l) > 10 )); then
            echo "$out" | sed 10q
            echo "[...]"
        else
            echo "$out"
        fi
        echo
    done
done

