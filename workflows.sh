#!/usr/bin/env bash

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
Run various commands against DWS workflows

Usage: $0 COMMAND [ARGS...]

Commands:
    advance WORKFLOW [--no-wait]        Advance the desired state of a workflow to the next state. If
                                        the workflow is not currently in a ready state, wait up to 60s
                                        for the workflow to become ready prior to advancing state. Use
                                        the '--no-wait' option to bypass the wait check and fail if the
                                        workflow is not currently ready.

    teardown [WORKFLOW]... [--all]      Set the desired state on a series of workflows to Teardown.
                                        This does not guarentee the Teardown state is achieved.
                                        Use the '--all' option to teardown all workflows in a system.
EOF
}

if [ $# -lt 1 ]; then
    usage
    exit 1
fi

declare -A NEXTSTATEARRAY=(["Proposal"]="Setup" ["Setup"]="DataIn" ["DataIn"]="PreRun" ["PreRun"]="DataOut" ["DataOut"]="Teardown" ["Teardown"]="Teardown")

case $1 in
    advance)
        WORKFLOW=${2}
        if [ ${!#} = "--no-wait" ]; then
            if [ "$(kubectl get workflow.dws/"$WORKFLOW" --output=custom-columns=:.status.ready --no-headers)" = "false" ]; then
                echo "ERROR: Workflow $WORKFLOW not ready. Aborting." >&2
                exit 1
            fi
        fi

        TIMEOUT=60s
        if ( timeout $TIMEOUT kubectl get workflow.dws/"$WORKFLOW" --output=custom-columns=:.status.ready --no-headers --watch & ) | grep -q "true"; then
            STATE=$(kubectl get workflows.dws/"$WORKFLOW" --output=custom-columns=:.status.state --no-headers)
            NEXTSTATE=${NEXTSTATEARRAY[$STATE]}
            echo "Advancing workflow $WORKFLOW to state $NEXTSTATE"
            kubectl patch --type merge workflows.dws/"$WORKFLOW" --patch "$(printf '{"spec": {"desiredState": "%s"}}' "$NEXTSTATE")"
        else
            echo "ERROR: Workflow $WORKFLOW not ready after $TIMEOUT. Aborting." >&2
            exit 1
        fi
        ;;
    teardown)
        if [ ${!#} = "--all" ]; then
            readarray -t WORKFLOWS < <(kubectl get workflows.dws --no-headers | awk '{print $1}')
        elif [ $# -gt 1 ]; then
            WORKFLOWS=( "${@:2}" )
        else
            echo "ERROR: WORKFLOW argument(s) required; or use '--all'" >&2
            usage
            exit 1
        fi

        for WORKFLOW in "${WORKFLOWS[@]}"; do
            kubectl patch --type merge workflow.dws/"$WORKFLOW" --patch '{"spec": {"desiredState": "Teardown"}}'
        done
        ;;
    *)
        usage
        exit 1
        ;;
esac
