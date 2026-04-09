#!/usr/bin/env bash

# Copyright 2026 Hewlett Packard Enterprise Development LP
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

# setup-release-env.sh — Source this to set up the release environment.
# Usage: source ./setup-release-env.sh [-B patch|minor|major]
#
# Exports: RELEASE_TYPE, PREVIOUS_RELEASE, NNF_RELEASE, RELEASE_USER,
#          RELEASE_REVIEWERS
# Defines: nnf_cmd, nnf_create_pr, nnf_add_reviewers, nnf_gh_repo

# NOTE: This file is sourced, not executed. Do not use 'set -euo pipefail'
# here — those options would persist in the caller's shell and cause any
# non-zero exit (even transient) to kill the terminal session.

# --- Parse -B flag -----------------------------------------------------------
OPTIND=1
RELEASE_TYPE="patch"
while getopts "B:" opt; do
    case $opt in
        B) RELEASE_TYPE="$OPTARG" ;;
        *) echo "Usage: source ./setup-release-env.sh [-B patch|minor|major]" >&2; return 1 ;;
    esac
done
export RELEASE_TYPE

# --- Compute versions --------------------------------------------------------
export PREVIOUS_RELEASE
PREVIOUS_RELEASE=$(gh release view --json tagName --jq '.tagName' \
    -R NearNodeFlash/nnf-deploy 2>&1)
if [[ -z "$PREVIOUS_RELEASE" || "$PREVIOUS_RELEASE" == *"release not found"* ]]; then
    echo "ERROR: Could not determine previous release from NearNodeFlash/nnf-deploy" >&2
    return 1
fi

case "$RELEASE_TYPE" in
    major) PERL_CMD='($x=$F[0])=~s/^v//; printf("v%s", join(".", $x+1, 0, 0))' ;;
    minor) PERL_CMD='print join(".", $F[0], $F[1]+1, 0)' ;;
    patch) PERL_CMD='print join(".", $F[0], $F[1], $F[2]+1)' ;;
    *)     echo "ERROR: Invalid release type '$RELEASE_TYPE'" >&2; return 1 ;;
esac
export NNF_RELEASE
NNF_RELEASE=$(echo "$PREVIOUS_RELEASE" | perl -F'/\./' -ane "$PERL_CMD")

# --- Detect operator & reviewers --------------------------------------------
RELEASE_USER=$(gh api user --jq '.login' 2>&1)
CODEOWNERS_FILE="../../.github/CODEOWNERS"
if [[ ! -f "$CODEOWNERS_FILE" ]]; then
    echo "ERROR: $CODEOWNERS_FILE not found. Run from tools/release-all/ inside an nnf-deploy clone." >&2
    return 1
fi
REVIEWER_POOL=$(grep '^\*' "$CODEOWNERS_FILE" | tr -s ' ' | cut -d' ' -f2- | tr ' ' '\n' | sed 's/@//' | paste -sd, -)
export RELEASE_REVIEWERS
RELEASE_REVIEWERS=$(echo "$REVIEWER_POOL" | tr ',' '\n' \
    | grep -v "$RELEASE_USER" | paste -sd, -)

if [[ -z "$RELEASE_REVIEWERS" ]]; then
    echo "WARNING: RELEASE_REVIEWERS is empty — you are the only code owner in CODEOWNERS." >&2
    echo "         You will need to assign reviewers manually for each PR." >&2
fi

# --- Helper: run release-all.sh with auto -M and -B -------------------------
# Usage: nnf_cmd <phase> <repo>
nnf_cmd() {
    local phase="$1" repo="$2"
    local args=() _rc
    [[ "$repo" == "nnf_sos" ]] && args+=("-M")
    ./release-all.sh -B "$RELEASE_TYPE" -P "$phase" -R "$repo" "${args[@]}" 2>&1 | cat
    # bash uses PIPESTATUS[0]; zsh uses pipestatus[1] (lowercase, 1-indexed)
    _rc=${PIPESTATUS[0]:-${pipestatus[1]}}
    return "${_rc:-0}"
}

# --- Helper: create-pr + capture PR number -----------------------------------
# Usage: nnf_create_pr <repo>
#   Sets PR_NUMBER as a side effect.
nnf_create_pr() {
    local repo="$1"
    local args=()
    [[ "$repo" == "nnf_sos" ]] && args+=("-M")
    local output
    output=$(./release-all.sh -B "$RELEASE_TYPE" -P create-pr -R "$repo" "${args[@]}" 2>&1 | tee /dev/tty)
    PR_NUMBER=$(echo "$output" | grep -o 'https://github.com/[^/ ]*/[^/ ]*/pull/[0-9]*' | awk -F'/' '{print $NF}' | head -n1)
    export PR_NUMBER
    echo "PR number: $PR_NUMBER"
}

# --- Helper: look up the GitHub <owner>/<repo> for a script name -------------
# Usage: nnf_gh_repo <script_name>
nnf_gh_repo() {
    local repo="$1"
    case "$repo" in
        dws)                  echo "DataWorkflowServices/dws" ;;
        lustre_csi_driver)    echo "HewlettPackard/lustre-csi-driver" ;;
        lustre_fs_operator)   echo "NearNodeFlash/lustre-fs-operator" ;;
        nnf_mfu)              echo "NearNodeFlash/nnf-mfu" ;;
        nnf_ec)               echo "NearNodeFlash/nnf-ec" ;;
        nnf_storedversions_maint) echo "NearNodeFlash/nnf-storedversions-maint" ;;
        nnf_sos)              echo "NearNodeFlash/nnf-sos" ;;
        nnf_dm)               echo "NearNodeFlash/nnf-dm" ;;
        nnf_integration_test) echo "NearNodeFlash/nnf-integration-test" ;;
        nnf_deploy)           echo "NearNodeFlash/nnf-deploy" ;;
        nnf_doc)              echo "NearNodeFlash/NearNodeFlash.github.io" ;;
        *)                    echo "UNKNOWN/$repo" ;;
    esac
}

# --- Helper: assign reviewers to the current PR ------------------------------
# Usage: nnf_add_reviewers <repo>
#   Requires PR_NUMBER to be set (via nnf_create_pr).
nnf_add_reviewers() {
    local repo="$1"
    local gh_repo
    gh_repo=$(nnf_gh_repo "$repo")
    gh pr edit "$PR_NUMBER" --add-reviewer "$RELEASE_REVIEWERS" \
        -R "$gh_repo" 2>&1 | cat
}

# --- Summary -----------------------------------------------------------------
echo "──────────────────────────────────────"
echo "  Previous release : $PREVIOUS_RELEASE"
echo "  New release      : $NNF_RELEASE"
echo "  Release type     : $RELEASE_TYPE"
echo "  Operator         : $RELEASE_USER"
echo "  Reviewers        : $RELEASE_REVIEWERS"
echo "──────────────────────────────────────"
