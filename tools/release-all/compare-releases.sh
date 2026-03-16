#!/bin/bash

# Copyright 2024-2026 Hewlett Packard Enterprise Development LP
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

PROG=$(basename "$0")

BOLD=$(tput bold)
NORMAL=$(tput sgr0)

do_fail() {
    echo "${BOLD}$*$NORMAL"
    exit 1
}

msg(){
    local msg="$1"

    echo "${BOLD}${msg}${NORMAL}"
}

NNF_URL="git@github.com:NearNodeFlash/nnf-deploy.git"

WORKSPACE="workingspace"
DISPLAY_DIFF=false
IMAGES_ONLY=false

usage() {
    echo "Usage: $PROG [-h]"
    echo "       $PROG [-d] [-i] [-w workspace_dir] [-u nnf_deploy_url] <VERSION1> <VERSION2>"
    echo
    echo "  VERSION1          Starting version for comparison. This may be"
    echo "                    a tag such as 'v0.1.7'."
    echo "  VERSION2          Ending version for comparison. This may be"
    echo "                    a tag such as 'v0.1.8'."
    echo "  -d                Display the full diff output."
    echo "  -i                Display only image version changes."
    echo "  -w workspace_dir  Name for working directory. Default: '$WORKSPACE'"
    echo "  -u nnf_deploy_url URL to the nnf-deploy repo. Default: '$NNF_URL'"
    echo
}

while getopts "dhiw:u:" opt; do
    case $opt in
    d)
        DISPLAY_DIFF=true
        ;;
    i)
        IMAGES_ONLY=true
        ;;
    u)
        NNF_URL="$OPTARG"
        ;;
    w)
        WORKSPACE="$OPTARG"
        ;;
    h)
        usage
        exit 0
        ;;
    :)
        echo "Option -$OPTARG requires an argument"
        exit 1
        ;;
    ?)
        echo "Invalid option -$OPTARG"
        exit 1
        ;;
    esac
done
shift $((OPTIND-1))
# Remaining args in: "$@"
if [[ $# -lt 2 ]]; then
    echo "Expected two versions to compare"
    exit 1
fi
if [[ $# -gt 2 ]]; then
    echo "Unexpected extra args: $*"
fi
ver1="$1"
ver2="$2"
v1tar="manifests-$ver1.tar"
v2tar="manifests-$ver2.tar"

if [[ ! -d $WORKSPACE ]]; then
    mkdir "$WORKSPACE" || do_fail "Cannot create workspace dir: $WORKSPACE"
fi
cd "$WORKSPACE" || do_fail "Unable to cd into $WORKSPACE"

prep_for_manifest() {
    local ver="$1"
    local vtar="$2"

    if [[ -f $vtar ]]; then
        rm "$vtar" || do_fail "Cannot remove old tarfile $vtar"
    fi

    if [[ -d $ver ]]; then
        rm -rf "$ver"
    fi

    if ! gh release view -R "$NNF_URL" "$ver" > /dev/null 2>&1; then
        do_fail "Release $ver does not exist"
    fi
}

fetch_and_unpack_manifest() {
    local ver="$1"
    local vtar="$2"

    gh release download -R "$NNF_URL" -O "$vtar" -p manifests.tar "$ver" || do_fail "Unable to find manifests.tar for $ver"
    [[ -s "$vtar" ]] || do_fail "Downloaded tarball $vtar is empty"

    mkdir "$ver" || do_fail "Cannot mkdir $ver"
    cd "$ver"
    tar xfo "../$vtar" || do_fail "Unable to extract tar $vtar"
    local count
    count=$(find . -type f | wc -l)
    [[ $count -gt 0 ]] || do_fail "No files were extracted from $vtar"
    cd ..
}

manifest() {
    local ver="$1"
    local vtar="$2"

    prep_for_manifest "$ver" "$vtar"
    fetch_and_unpack_manifest "$ver" "$vtar"
}

manifest "$ver1" "$v1tar"
manifest "$ver2" "$v2tar"

diff_file="manifest-$ver1-to-$ver2.diff"
rc=0
diff -uNr "$ver1" "$ver2" > "$diff_file" || rc=$?
if [[ $rc -eq 0 ]]; then
    msg "No differences found between $ver1 and $ver2"
    rm -f "$diff_file"
elif [[ $rc -eq 1 ]]; then
    lines=$(wc -l < "$diff_file" | tr -d ' ')
    changed_files=""
    changed_files=$(diff -rq "$ver1" "$ver2") || true
    changed=$(echo "$changed_files" | wc -l | tr -d ' ')
    msg "Manifest diffs for $ver1 to $ver2 are in $WORKSPACE/$diff_file ($lines lines, $changed files changed)"
    msg "Changed files:"
    echo "$changed_files"

    if [[ $IMAGES_ONLY == true ]]; then
        msg ""
        msg "Image version changes:"
        grep -E '^[+-].*image:.*:' "$diff_file" | grep -v '^[+-][+-][+-]' || echo "  (no image changes found)"
    fi

    if [[ $DISPLAY_DIFF == true ]]; then
        msg ""
        msg "Full diff:"
        cat "$diff_file"
    fi
else
    do_fail "diff encountered an error (rc=$rc) comparing $ver1 and $ver2"
fi

