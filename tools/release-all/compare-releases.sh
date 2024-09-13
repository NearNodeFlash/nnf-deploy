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

usage() {
    echo "Usage: $PROG [-h]"
    echo "       $PROG [-w workspace_dir] [-u nnf_deploy_url] <VERSION1> <VERSION2>"
    echo
    echo "  VERSION1          Starting version for comparison. This may be"
    echo "                    a tag such as 'v0.1.7'."
    echo "  VERSION2          Ending version for comparison. This may be"
    echo "                    a tag such as 'v0.1.8'."
    echo "  -w workspace_dir  Name for working directory. Default: '$WORKSPACE'"
    echo "  -u nnf_deploy_url URL to the nnf-deploy repo. Default: '$NNF_URL'"
    echo
}

while getopts "hw:u:" opt; do
    case $opt in
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

    if ! gh release view "$ver" > /dev/null 2>&1; then
        do_fail "Release $ver does not exist"
    fi
}

fetch_and_unpack_manifest() {
    local ver="$1"
    local vtar="$2"

    gh release download -R "$NNF_URL" -O "$vtar" -p manifests.tar "$ver" || do_fail "Unable to find manifests.tar for $ver"

    mkdir "$ver" || do_fail "Cannot mkdir $ver"
    cd "$ver"
    tar xfo "../$vtar" || do_fail "Unable to extract tar $vtar"
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
diff -uNr "$ver1" "$ver2" > "$diff_file" || echo
msg "Manifest diffs for $ver1 to $ver2 are in $WORKSPACE/$diff_file"

