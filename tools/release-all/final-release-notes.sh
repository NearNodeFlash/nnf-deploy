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

PROG=$(basename "$0")

NNF_DOC_URL='git@github.com:NearNodeFlash/NearNodeFlash.github.io.git'
NNF_DEPLOY_URL='git@github.com:NearNodeFlash/nnf-deploy.git'

BOLD=$(tput bold)
NORMAL=$(tput sgr0)

do_fail() {
    echo "${BOLD}$*$NORMAL"
    exit 1
}

WORKSPACE="workingspace"
TARGET_NNF_RELEASE=
SKIP_DOCS=
DO_COMMIT=

usage() {
    echo "Usage: $PROG [-h]"    
    echo "       $PROG <-r nnf_release> [-C] [-D] [-w workspace_dir]"
    echo
    echo "  -C                Commit the new release notes. First, run without"
    echo "                    this so you can review the notes."
    echo "  -D                Skip the docs repo. If the docs repo did not have"
    echo "                    updates for this NNF release, then specify this"
    echo "                    option to skip it."
    echo "  -r nnf_release    Latest NNF release tag, including the leading 'v'."
    echo "                    This must refer to a completed NNF release,"
    echo "                    including the docs repo, and it must be the most"
    echo "                    recent release."
    echo "                    By specifying it here, it is used as a sanity check."
    echo "  -w workspace_dir  Name for working directory. Default: '$WORKSPACE'"
    echo
}

while getopts "CDr:w:h" opt; do
    case $opt in
    C)
        DO_COMMIT=yes
        ;;
    D)
        SKIP_DOCS=yes
        ;;
    r)
        TARGET_NNF_RELEASE="$OPTARG"
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
if [[ -n $* ]]; then
    echo "Unexpected extra args: $*"
    exit 1
fi

if [[ -z $TARGET_NNF_RELEASE ]]; then
    echo "Arg -r is required."
    exit 1
fi

if ! which yq > /dev/null 2>&1; then
    echo "Unable to find yq in PATH"
    exit 1
fi

if [[ ! -d $WORKSPACE ]]; then
    mkdir "$WORKSPACE" || do_fail "Cannot create workspace dir: $WORKSPACE"
fi
cd "$WORKSPACE" || do_fail "Unable to cd into $WORKSPACE"

NNF_DEPLOY_RELEASE=
RELEASE_PAGE="$(PWD)/RELEASE_PAGE.md"

msg(){
    local msg="$1"

    echo "${BOLD}${msg}${NORMAL}"
}

verify_clean_workarea() {
    local indent="$1"

    if [[ $(git status -s | wc -l) -gt 0 ]]; then
        do_fail "${indent}Repo isn't clean. I'm confused."
    fi
}

get_default_branch() {
    local repo_short_name="$1"
    local default_branch=master

    [[ $repo_short_name = nnf_doc ]] && default_branch=main
    echo "$default_branch"
}

get_repo_dir_name() {
    local repo_url="$1"
    local bn

    bn=$(basename "$repo_url")
    echo "${bn%.git}"
}

check_submodules() {
    local branch="$1"
    local already_initialized="$2"
    local indent="$3"

    [[ ! -f .gitmodules ]] && return

    if [[ $already_initialized != true ]] && [[ $(git submodule status | grep -c -v -E '^-') -gt 0 ]]; then
        do_fail "${indent}Expected submodules to be uninitialized."
    fi

    git submodule update --init || do_fail "${indent}Failure while checking out submodules."
    if [[ $(git submodule status | grep -c -v -E '^ ') -gt 0 ]]; then
        do_fail "${indent}Submodules did not checkout clean."
    fi
}

clone_checkout_fresh_workarea() {
    local repo_name="$1"
    local repo_url="$2"
    local branch="$3"
    local indent="$4"

    if [[ -z $USE_EXISTING_WORKAREA ]]; then
        git clone "$repo_url" || do_fail "${indent}Failure while cloning"
    fi
    cd "$repo_name" || do_fail "${indent}Unable to cd into $repo_name."
    if [[ $branch != "main" && $branch != "master" ]]; then
        git checkout "$branch" || do_fail "${indent}Failure checking out $branch."
        git pull || do_fail "${indent}Failure pulling latest commits."
        if [[ -f .gitmodules ]]; then
            git submodule update --init || do_fail "${indent}Failure updating submodules for $branch."
        fi
    fi
}

begin_with_clean_workarea() {
    local repo_name="$1"
    local indent="$2"

    if [[ -d "$repo_name" ]]; then
        if [[ -z $USE_EXISTING_WORKAREA ]]; then
            msg "${indent}Removing existing $repo_name"
            rm -rf "$repo_name"
            if [[ -d "$repo_name" ]]; then
                do_fail "${indent}Unable to begin with clean repo for $repo_name."  
            fi
        else
            msg "${indent}WARNING:"
            msg "${indent}WARNING: Using existing $repo_name workarea!"
            msg "${indent}WARNING: Some things may slip past my sanity checks!"
            msg "${indent}WARNING:"
        fi
    fi
}

find_latest_release(){
    local url="$1"
    local latest_tag

    latest_tag=$(gh release list -R "$url" --json isLatest,tagName | jq -rM '.[]|select(.isLatest==true)|.tagName')
    # Strip the leading 'v'.
    latest_tag="${latest_tag#v}"
    echo "$latest_tag"
}

release_switch_submodules() {
    local new_branch="$1"
    local submod_branch="$2"
    local indent="$3"

    msg "${indent}Checking submodules"
    if ! git status | grep -q -E '^On branch '"$new_branch"'$'; then
        if ! git status | grep -q -E '^HEAD detached at '"$new_branch"'$'; then
            do_fail "${indent}Not on expected release branch $new_branch"
        fi
    fi

    check_submodules "$submod_branch" "true" "$indent"
}

check_release_vX() {
    local repo_short_name="$1"
    local repo_name="$2"
    local repo_url="$3"
    local branch="$4"
    local indent="  "

    default_branch=$(get_default_branch "$repo_short_name")
    msg "Repo $repo_name/$branch:"
    begin_with_clean_workarea "$repo_name" "$indent"

    clone_checkout_fresh_workarea "$repo_name" "$repo_url" "$branch" "$indent"

    gh_latest_release=$(find_latest_release "$repo_url")
    latest_release=$(git describe --match="v*" --abbrev=0 HEAD) || do_fail "${indent}Failure getting latest release tag."
    if [[ "v$gh_latest_release" != "$latest_release" ]]; then
        do_fail "${indent}Latest release found in GH is $gh_latest_release, but latest found in repo is $latest_release"
    fi

    # Previous release:
    prev_release=$(git log --oneline --decorate --merges | grep 'tag: v' | sed 2q | tail -1 | sed -e 's/^.*tag: \(v.*[0-9]\).*Merge release .*$/\1/')

    echo
    msg "${indent}Latest release is: $latest_release"
    msg "${indent}Previous release was: $prev_release"
    echo

    if [[ $repo_short_name == nnf_deploy ]]; then
        if [[ $latest_release != "$TARGET_NNF_RELEASE" ]]; then
            do_fail "${indent}Latest release does not match specified target of $TARGET_NNF_RELEASE."
        fi
        NNF_DEPLOY_RELEASE="$latest_release"
    fi

    verify_clean_workarea "$indent"

    if ! git checkout "$latest_release"; then
        do_fail "${indent}Failure checking out $latest_release."
    fi
    release_switch_submodules "$latest_release" "$branch" "$indent"
    verify_clean_workarea "$indent"

    echo
    msg "${indent}Submodule versions in $latest_release"
    echo
    git submodule status
    echo

    local latest_subdirs_txt=../latest-subdirs.txt
    git submodule status | awk '{print $2" "$3}' | sed -e 's/[()]//g' > "$latest_subdirs_txt"

    msg "${indent}Collect release notes for $repo_name $latest_release."
    if ! body=$(gh release view "$latest_release" --json body); then
        do_fail "${indent}Failure viewing release $latest_release"
    fi

    if [[ $repo_short_name == nnf_deploy ]]; then
        # If nnf-deploy, then begin a new release page.
        echo "## NNF release $latest_release" > "$RELEASE_PAGE"
    fi
    # shellcheck disable=SC2129
    echo >> "$RELEASE_PAGE"

    echo "### $repo_name $latest_release" >> "$RELEASE_PAGE"
    echo >> "$RELEASE_PAGE"
    echo "$body" | sed 's/##/####/' | jq -rM .body >> "$RELEASE_PAGE"

    if [[ $repo_short_name == nnf_deploy ]]; then
        submods=$(grep submodule .gitmodules | tr '"' ' ' | awk '{print $2}')
        not_modified=
        for submod in $submods; do
            if [[ $(git diff "$prev_release" -- "$submod" | wc -l) -eq 0 ]]; then
                not_modified="$submod $not_modified"
                continue
            fi
            submod_rel=$(grep -E "^$submod " $latest_subdirs_txt | awk '{print $2}')
            submod_url=$(grep -E '^\turl = ' .git/modules/"$submod"/config | awk '{print $3}')
            msg "${indent}Collect release notes for $submod $submod_rel."
            if ! body=$(gh release view --repo "$submod_url" "$submod_rel" --json body); then
                do_fail "${indent}Failure viewing $submod release $submod_rel"
            fi
            # shellcheck disable=SC2129
            echo >> "$RELEASE_PAGE"
            echo "### $submod $submod_rel" >> "$RELEASE_PAGE"
            echo >> "$RELEASE_PAGE"
            echo "$body" | sed 's/##/####/' | jq -rM .body >> "$RELEASE_PAGE"
        done
        if [[ -n $not_modified ]]; then
            echo
            msg "${indent}The following submodules were not modified in this release:"
            msg "${indent}$not_modified"
            echo
        fi
    fi

    echo
    cd ..
}

dep_repo_short_name="nnf_deploy"
dep_url="$NNF_DEPLOY_URL"
dep_name=$(get_repo_dir_name "$dep_url")
check_release_vX "$dep_repo_short_name" "$dep_name" "$dep_url" "releases/v0"

if [[ -z $SKIP_DOCS ]]; then
    doc_repo_short_name="nnf_doc"
    doc_url="$NNF_DOC_URL"
    doc_name=$(get_repo_dir_name "$doc_url")
    check_release_vX "$doc_repo_short_name" "$doc_name" "$doc_url" "releases/v0"
fi

if [[ -n $NNF_DEPLOY_RELEASE && -f $RELEASE_PAGE ]]; then
    if [[ -n $DO_COMMIT ]]; then
        gh release edit "$NNF_DEPLOY_RELEASE" --notes-file "$RELEASE_PAGE"
    else
        msg "Review notes in $RELEASE_PAGE."
    fi
else
    msg "No release notes were created."
fi

