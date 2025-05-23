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

PROG=$(basename "$0")

# Crude associative-array-ish thingy that works in bash v3 for Mac.
repomap_keys[0]=dws
repomap_keys[1]=lustre_csi_driver
repomap_keys[2]=lustre_fs_operator
repomap_keys[3]=nnf_mfu
repomap_keys[4]=nnf_ec
repomap_keys[5]=nnf_sos
repomap_keys[6]=nnf_dm
repomap_keys[7]=nnf_integration_test
repomap_keys[8]=nnf_deploy
repomap_keys[9]=nnf_doc

declare "repomap_${repomap_keys[0]}"='git@github.com:DataWorkflowServices/dws.git'
declare "repomap_${repomap_keys[1]}"='git@github.com:HewlettPackard/lustre-csi-driver.git'
declare "repomap_${repomap_keys[2]}"='git@github.com:NearNodeFlash/lustre-fs-operator.git'
declare "repomap_${repomap_keys[3]}"='git@github.com:NearNodeFlash/nnf-mfu.git'
declare "repomap_${repomap_keys[4]}"='git@github.com:NearNodeFlash/nnf-ec.git'
declare "repomap_${repomap_keys[5]}"='git@github.com:NearNodeFlash/nnf-sos.git'
declare "repomap_${repomap_keys[6]}"='git@github.com:NearNodeFlash/nnf-dm.git'
declare "repomap_${repomap_keys[7]}"='git@github.com:NearNodeFlash/nnf-integration-test.git'
declare "repomap_${repomap_keys[8]}"='git@github.com:NearNodeFlash/nnf-deploy.git'
declare "repomap_${repomap_keys[9]}"='git@github.com:NearNodeFlash/NearNodeFlash.github.io.git'

getter() {
    # Getter for the associative-array-ish thingy that works in bash v3 for Mac.
    local arr="$1"
    local key="$2"
    local parts="${arr}_$key"
    printf '%s' "${!parts}"
}

BOLD=$(tput bold)
NORMAL=$(tput sgr0)

do_fail() {
    echo "${BOLD}$*$NORMAL"
    exit 1
}

list_repos() {
    echo "Repo names:"
    echo "${repomap_keys[@]}" | tr ' ' '\n' | sed -e 's/^/  /'
}

verify_repo_keys() {
    keylist="$1"
    for key in $keylist; do
        val=$(getter repomap "$key")
        if [[ -z $val ]]; then
            echo "Unrecognized repo name: $key"
            echo "To get a listing:"
            echo "    $PROG -L"
            exit 1
        fi
    done
}

REPO_LIST=""
SEMVER_BUMP="patch"
PUSH_BRANCH=""
WORKSPACE="workingspace"
PHASE="master"
ALLOW_VENDOR_MULTI_API=

usage() {
    echo "Usage: $PROG [-h]"
    echo "       $PROG [-L]"
    echo "       $PROG [-w workspace_dir] [-P phase] [-R repo-names] [-B part] [-x THINGS] [-M]"
    echo
    echo "  -B part           Indicates which part of the version to bump for"
    echo "                    the new release. Default: '$SEMVER_BUMP'."
    echo "                      'major' Bump the major part."
    echo "                      'minor' Bump the minor part."
    echo "                      'patch' Bump the patch part."
    echo "  -L                List recognized repo names, then exit."
    echo "                      This list shows you the proper order to go through"
    echo "                      when creating releases for the repos. Some repos"
    echo "                      have references to others, so the order matters."
    echo "  -P phase          Indicates which phase to run. Default: '$PHASE'."
    echo "                      'master'       Validate the master/main branches."
    echo "                      'release'      Create the release branches, but don't push."
    echo "                      'release-push' Create and push the release branches."
    echo "                      'create-pr'    Create PR for release branches."
    echo "                      'merge-pr'     Merge PR for release branches."
    echo "                      'tag-release'  Tag the release."
    echo "  -R repo_names     Comma-separated list of repo names to operate on."
    echo "                    If unspecified, then all repos will be used."
    echo "                    The phases that follow 'release' allow only one repo."
    echo "  -M                Allow multiple API versions to be vendored from a"
    echo "                    single peer module. Expect this to be an unusual"
    echo "                    case."
    echo "  -w workspace_dir  Name for working directory. Default: '$WORKSPACE'"
    echo "  -x THINGS         A list of colon-separated manual overrides."
    echo "                      'force-tag=vX.Y.Z'  Use tag vX.Y.Z during the tag-release"
    echo "                                          phase. Use this if you accidently did"
    echo "                                          a manual merge rather than using the"
    echo "                                          merge-pr phase above."
    echo
    echo "See README.md for detailed instructions"
}

while getopts "B:LP:R:Mw:x:h" opt; do
    case $opt in
    B)
        case $OPTARG in
        major|minor|patch) SEMVER_BUMP="$OPTARG" ;;
        *) echo "The -B arg takes  'major', 'minor', or 'patch'."
           exit 1
           ;;
        esac
        ;;
    L)
        list_repos
        exit 0
        ;;
    P)
        case $OPTARG in
        # Allow dash or underscore.
        master|release|create[_-]pr|merge[_-]pr|tag[_-]release)
            PHASE=$(echo "$OPTARG" | tr "\-" _)
            ;;
        release[_-]push)
            PHASE="release"
            PUSH_BRANCH=true
            ;;
        *) echo "The -P arg does not understand '$OPTARG'."
           exit 1
           ;;
        esac
        ;;
    R)
        # Legal variable names need dashes in place of underscores.
        REPO_LIST=$(echo "$OPTARG" | tr , " " | tr "\-" _)
        verify_repo_keys "$REPO_LIST"
        ;;
    w)
        WORKSPACE="$OPTARG"
        ;;
    M)
        ALLOW_VENDOR_MULTI_API=1
        ;;
    x)
        OVERRIDES=${OPTARG//:/ }
        bad_one=
        for override in $OVERRIDES; do
          if [[ "$override" =~ ^force\-tag=v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
              FORCE_TAG_VALUE=${override//force-tag=/}
              echo "Using force-tag: $FORCE_TAG_VALUE"
          else
              echo "Unrecognized -x option: $override"
              bad_one=yes
          fi
        done
        if [[ -n $bad_one ]]; then
            exit 1
        fi
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

if ! which yq > /dev/null 2>&1; then
    echo "Unable to find yq in PATH"
    exit 1
fi

if [[ -n $REPO_LIST ]]; then
    echo "Working on repos: $REPO_LIST"
else
    echo "Working on all repos."
    REPO_LIST="${repomap_keys[*]}"
fi

if [[ ! -d $WORKSPACE ]]; then
    mkdir "$WORKSPACE" || do_fail "Cannot create workspace dir: $WORKSPACE"
fi
cd "$WORKSPACE" || do_fail "Unable to cd into $WORKSPACE"

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

check_auto_gens() {
    local indent="$1"

    if [[ -f Makefile ]]; then
        msg "${indent}Checking generated files"
        if grep -qE '^manifests:' Makefile; then
            make manifests || do_fail "${indent}Failed: make manifests"
        fi
        if grep -qE '^generate:' Makefile; then
            make generate || do_fail "${indent}Failed: make generate"
        fi
        if grep -qE '^generate-go-conversions:' Makefile; then
            make generate-go-conversions || do_fail "${indent}Failed: make generate-go-conversions"
        fi
    fi
}

verify_crd_conversions() {
    local indent="$1"

    if [[ -f Makefile ]]; then
        if grep -qE '^verify-conversions:' Makefile; then
            msg "${indent}Checking CRD conversions"
            make verify-conversions || do_fail "${indent}CRD conversion verifier failed"
        fi
    fi
}

# Peer modules refers to the other DWS/NNF modules listed in go.mod.
check_peer_modules() {
    local indent="$1"

    [[ ! -f go.mod ]] && return

    peer_modules=$(grep -e DataWorkflowServices -e NearNodeFlash -e HewlettPackard go.mod | grep -v -e module -e structex | awk '{print $1"@master"}' | paste -s -)
    if [[ -n $peer_modules ]]; then
        msg "${indent}Checking peer modules: $peer_modules"

        # shellcheck disable=SC2086
        go get $peer_modules || do_fail "${indent}Failure getting modules: $peer_modules"
        go mod tidy || do_fail "${indent}Failure in go mod tidy"
        if [[ -d vendor ]]; then
            go mod vendor || do_fail "${indent}Failure in go mod vendor"
        fi

        if [[ $(git status -s | wc -l) -gt 0 ]]; then
            msg "${indent}Peer modules are behind."
            msg "${indent}Update the modules and create a PR. I used:"
            echo
            echo "go get $peer_modules"
            echo
            exit 1
        fi
    fi
}

# Peer modules refers to the other DWS/NNF modules listed in go.mod. This
# verifies that we vendored only one API version for each peer module.
check_peer_module_api_count() {
    local indent="$1"

    [[ ! -f go.mod ]] && return

    peer_modules=$(grep -e DataWorkflowServices -e NearNodeFlash -e HewlettPackard go.mod | grep -v -e module -e structex | awk '{print $1}')
    if [[ -n $peer_modules ]]; then
        echo
        local found
        for mod in $peer_modules; do
            modpath="vendor/$mod/api"
            if [[ -d $modpath ]]; then
                if [[ $(/bin/ls -1 "$modpath" | wc -l) -gt 1 ]]; then
                    msg "${indent}Vendored multiple APIs in $modpath."
                    if [[ -z $ALLOW_VENDOR_MULTI_API ]]; then
                        msg "${indent}Update the code to use only one."
                        msg "${indent}If this condition is expected, then override this check with -M."
                        exit 1
                    fi
                    found=1
                fi
            fi
        done
        if [[ -n $found ]]; then
            echo
            msg "${indent}Multiple APIs have been allowed with -M, continuing"
            echo
        fi
    fi
}

summarize_submodule_commits() {
    local indent="$1"

    [[ ! -f .gitmodules ]] && return

    if [[ $(git submodule status | grep -cE '^\+') -gt 0 ]]; then
        upd_submods=$(git submodule status | grep -E '^\+' | awk '{print $2}')
        for mod in $upd_submods; do
            echo
            msg "${indent}Updates found in submodule $mod"
            prev_commit=$(git diff "$mod" | grep -E '^-Subproject' | awk '{print $3}')
            pushd "$mod" > /dev/null || do_fail "${indent}Unable to summarize submodule $mod"
            git log --oneline "$prev_commit...HEAD" | cat
            popd > /dev/null || do_fail "${indent}Unable to popd from $mod"
        done
        echo
    fi
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

    git submodule foreach git checkout "$branch" || do_fail "${indent}Failure during checkout of $branch in submodules"
    git submodule foreach git pull || do_fail "${indent}Failure pulling latest commits in submodules"

    if [[ $already_initialized != true ]] && [[ $(git status -s | wc -l) -gt 0 ]]; then
        summarize_submodule_commits "$indent"
        msg "${indent}Submodules are not up to date."
        msg "${indent}Update the modules and create a PR."
        exit 1
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

semver_bump() {
    local latest_release="$1"
    local new_release=""

    case $SEMVER_BUMP in
    major)
        # The math on $F[0] means it'll drop the leading "v", so we have to
        # put it back.
        new_release=$(echo "$latest_release" | perl -F'/\./' -ane 'printf("v%s", join(".", $F[0]+1, 0, 0))')
        ;;
    minor)
        new_release=$(echo "$latest_release" | perl -F'/\./' -ane 'print join(".", $F[0], $F[1]+1, 0)')
        ;;
    *)
        new_release=$(echo "$latest_release" | perl -F'/\./' -ane 'print join(".", $F[0], $F[1], $F[2]+1)')
        ;;
    esac

    echo "$new_release"
}

update_kustomization_file(){
    local kdir="$1"
    local name="$2"
    local release_ver="$3"
    local indent="$4"

    make kustomize || do_fail "${indent}Unable to retrieve kustomize"
    pushd "$kdir" > /dev/null || do_fail "${indent}Unable to change to $kdir"
    image_name=$(yq -rM eval '.images[]|select(.name=="'"$name"'")|.newName' kustomization.yaml)
    ~-/bin/kustomize edit set image "$name=$image_name:$release_ver" || do_fail "${indent}Unable to set the $name version in kustomization.yaml"
    popd > /dev/null || do_fail "${indent}Unable to popd from $kdir"
}

find_latest_release(){
    local repo_short_name="$1"
    local indent="$2"
    local url
    local latest_tag

    url=$(getter repomap "$repo_short_name")
    latest_tag=$(gh release list -R "$url" --json isLatest,tagName | jq -rM '.[]|select(.isLatest==true)|.tagName')
    # Strip the leading 'v'.
    latest_tag="${latest_tag#v}"
    echo "$latest_tag"
}

# Update versions where the release is referring to itself.
update_own_release_references() {
    local repo_short_name="$1"
    local vrelease_ver="$2"
    local indent="$3"
    local release_ver

    # Strip the leading 'v'.
    release_ver="${vrelease_ver#v}"

    case "$repo_short_name" in
    dws)
        k_yaml="config/manager"
        update_kustomization_file "$k_yaml" controller "$release_ver" "$indent"
        git add "$k_yaml"
        ;;
    lustre_csi_driver)
        k_yaml="deploy/kubernetes/base"
        update_kustomization_file "$k_yaml" controller "$release_ver" "$indent"
        git add "$k_yaml"

        # This one also has a helm chart. Use `yq -i e` to do an in-place edit
        # with the yq that is written in Go. The yq written in Python uses
        # `-yi`.
        chart="charts/lustre-csi-driver/values.yaml"
        yq -i e -M '(.deployment.tag) = "'"$release_ver"'"' "$chart" || do_fail "${indent}Unable to update release version in helm chart"
        git add "$chart"
        ;;
    lustre_fs_operator)
        k_yaml="config/manager"
        update_kustomization_file "$k_yaml" controller "$release_ver" "$indent"
        git add "$k_yaml"
        ;;
    nnf_sos)
        k_yaml="config/manager"
        update_kustomization_file "$k_yaml" controller "$release_ver" "$indent"
        git add "$k_yaml"
        ;;
    nnf_dm)
        k_yaml="config/manager"
        update_kustomization_file "$k_yaml" controller "$release_ver" "$indent"
        git add "$k_yaml"
        ;;
    esac

    if [[ $(git status -s | wc -l) -gt 0 ]]; then
        git commit -s -m "Update own release references" || do_fail "${indent}Failure updating own release references"
    fi
}

# Update references to the nnf-mfu release.
update_nnf_mfu_release_references() {
    local repo_short_name="$1"
    local indent="$2"
    local nnf_mfu_release

    case "$repo_short_name" in
    nnf_sos|nnf_dm|nnf_deploy)
        ;;
    *)
        return
        ;;
    esac

    nnf_mfu_release=$(find_latest_release nnf_mfu "$indent")
    if [[ -z $nnf_mfu_release ]]; then
        do_fail "${indent}Unable to get nnf-mfu releases"
    fi
    msg "${indent}Current latest nnf-mfu release is $nnf_mfu_release"

    case "$repo_short_name" in
    nnf_sos|nnf_dm)
        # Point the Makefile at the latest nnf-mfu version.
        sed -i.bak -e 's/^\(NNFMFU_VERSION \?= \).*/\1'"$nnf_mfu_release"'/' Makefile
        if [[ $(git status -s Makefile | wc -l) -gt 0 ]]; then
            if [[ $(diff -U0 Makefile.bak Makefile | wc -l) -ne 5 ]]; then
                do_fail "${indent}Unexpected change while setting nnf-mfu release in Makefile"
            fi
            git add Makefile
        fi
        rm -f Makefile.bak
        ;;
    esac

    case "$repo_short_name" in
    nnf_dm)
        k_yaml="config/manager"
        update_kustomization_file "$k_yaml" nnf-mfu "$nnf_mfu_release" "$indent"
        git add "$k_yaml"

        # Point the Dockerfile at the latest nnf-mfu version.
        sed -i.bak -e 's/^\(ARG NNFMFU_VERSION=\).*/\1'"$nnf_mfu_release"'/' Dockerfile
        if [[ $(git status -s Dockerfile | wc -l) -gt 0 ]]; then
            if [[ $(diff -U0 Dockerfile.bak Dockerfile | wc -l) -ne 5 ]]; then
                do_fail "${indent}Unexpected change while setting nnf-mfu release in Dockerfile"
            fi
            git add Dockerfile
        fi
        rm -f Dockerfile.bak
        ;;
    nnf_deploy)
        yq -i e -M '(.buildConfiguration.env[] | select(.name=="NNFMFU_VERSION") | .value) = "'"$nnf_mfu_release"'"' config/repositories.yaml
        git add config/repositories.yaml
        ;;
    esac

    if [[ $(git status -s | wc -l) -gt 0 ]]; then
        git commit -s -m "Update nnf-mfu release references" || do_fail "${indent}Failure updating nnf-mfu release references"
    fi
}

# Update nnf-docs for the latest nnf-deploy release version.
update_nnf_release_reference() {
    local indent="$1"
    local yml="mkdocs.yml"

    [[ ! -f $yml ]] && return
    nnf_deploy_release=$(find_latest_release nnf_deploy "$indent")
    if [[ -z $nnf_deploy_release ]]; then
        do_fail "${indent}Unable to get nnf-deploy releases"
    fi
    msg "${indent}Current latest nnf-deploy release is $nnf_deploy_release"
    sed -i.bak -e 's/^# Release: .*/# Release: '"$nnf_deploy_release"'/' $yml || do_fail "${indent}Failed to replace nnf-deploy release reference"
    rm -f $yml.bak
    git add $yml
    git commit -s -m "Update nnf-deploy release reference" || do_fail "${indent}Failure updating nnf-deploy release reference"
}

# Update lustre-fs-operator, lustre-csi-driver for
# nnf-deploy's config/repositories.yaml.
update_remote_release_references() {
    local indent="$1"
    local lustre_fs_release
    local lustre_csi_release

    lustre_fs_release=$(find_latest_release lustre_fs_operator "$indent")
    if [[ -z $lustre_fs_release ]]; then
        do_fail "${indent}Unable to get lustre-fs-operator releases"
    fi
    msg "${indent}Current latest lustre-fs-operator release is $lustre_fs_release"

    lustre_csi_release=$(find_latest_release lustre_csi_driver "$indent")
    if [[ -z $lustre_csi_release ]]; then
        do_fail "${indent}Unable to get lustre-csi-driver releases"
    fi
    msg "${indent}Current latest lustre-csi-driver release is $lustre_csi_release"

    yq -i e -M '(.repositories[] | select(.name=="lustre-csi-driver") | .remoteReference.build) = "'"v$lustre_csi_release"'"' config/repositories.yaml
    yq -i e -M '(.repositories[] | select(.name=="lustre-fs-operator") | .remoteReference.build) = "'"v$lustre_fs_release"'"' config/repositories.yaml
    git add config/repositories.yaml

    if [[ $(git status -s | wc -l) -gt 0 ]]; then
        git commit -s -m "Update lustre-fs-operator or lustre-csi-driver release references" || do_fail "${indent}Failure updating lustre-fs-operator or lustre-csi-driver release references"
    fi
}

release_switch_submodules() {
    local new_branch="$1"
    local submod_branch="$2"
    local indent="$3"

    msg "${indent}Checking submodules"
    if ! git status | grep -q -E '^On branch '"$new_branch"'$'; then
        do_fail "${indent}Not on expected release branch $new_branch"
    fi

    check_submodules "$submod_branch" "true" "$indent"
    # The submodule status should now show the latest release of each one.
    echo
    msg "${indent}Submodule status, pre-merge:"
    echo
    git submodule status
    echo
}

check_repo_master() {
    local repo_short_name="$1"
    local repo_name="$2"
    local repo_url="$3"
    local default_branch
    local indent="  "

    default_branch=$(get_default_branch "$repo_short_name")
    msg "Repo $repo_name/$default_branch:"
    begin_with_clean_workarea "$repo_name" "$indent"

    clone_checkout_fresh_workarea "$repo_name" "$repo_url" "$default_branch" "$indent"

    if [[ $repo_short_name != nnf_deploy ]] && [[ $repo_short_name != nnf_ec ]]; then
        check_auto_gens "$indent"
    fi
    verify_clean_workarea "$indent"
    verify_crd_conversions "$indent"

    check_peer_modules "$indent"
    verify_clean_workarea "$indent"
    check_peer_module_api_count "$indent"

    check_submodules master "false" "$indent"
    verify_clean_workarea "$indent"

    echo
    cd ..
}

check_repo_release_vX() {
    local repo_short_name="$1"
    local repo_name="$2"
    local repo_url="$3"
    local branch="$4"
    local default_branch
    local indent="  "
    local has_changes=false

    default_branch=$(get_default_branch "$repo_short_name")
    msg "Repo $repo_name/$branch:"
    msg "  Merge default branch $default_branch"
    begin_with_clean_workarea "$repo_name" "$indent"

    clone_checkout_fresh_workarea "$repo_name" "$repo_url" "$branch" "$indent"

    latest_release=$(git describe --match="v*" --abbrev=0 HEAD) || do_fail "${indent}Failure getting latest release tag."
    new_release=$(semver_bump "$latest_release")

    echo
    msg "${indent}Latest release is: $latest_release"
    msg "${indent}New release will be: $new_release"
    echo

    verify_clean_workarea "$indent"

    new_branch="release-$new_release"
    msg "${indent}Checking for existing pushed branch $repo_name/$new_branch."
    if git checkout "$new_branch"; then
        echo
        msg "${indent}Using existing pushed branch $repo_name/$new_branch."
        echo
    else
        msg "${indent}Creating new branch $repo_name/$new_branch."
        echo
        git checkout -b "$new_branch" || do_fail "${indent}Failure checking out $new_release branch."
    fi

    local not_clean=false
    if [[ -f .gitmodules ]]; then
        release_switch_submodules "$new_branch" "$branch" "$indent"
        # We're now pointing at the latest release of each submodule.
        echo
        msg "${indent}Expect messages about conflicts; I'll fix them."
        echo
    fi
    if ! git merge --signoff --stat --no-edit "$default_branch"; then
        if [[ -f .gitmodules ]]; then
            # The git-merge did not modify the submodules, but it did tell
            # us that it cannot merge them. We already have them pointed at
            # their latest releases, so add them as-is and complete the
            # merge.
            echo
            msg "${indent}Fixing conflicts now..."
            echo
            new_submods=$(git submodule status | grep -E '^U' | awk '{print $2}')
            for mod in $new_submods; do
                git add "$mod"
            done
            if ! git commit -s -m "Merge branch '$default_branch' into $new_branch"; then
                not_clean=true
            else
                echo
                msg "${indent}The conflicts have been fixed."
                msg "${indent}Submodule status:"
                echo
                git submodule status
                echo
            fi
        else
            not_clean=true
        fi
    fi

    if [[ $not_clean == true ]]; then
        echo
        do_fail "${indent}Merge is not clean: from $default_branch to $new_branch"
    fi

    if [[ -f .gitmodules ]] && [[ $(git submodule status | grep -cE '^\+') -gt 0 ]]; then
        msg "${indent}Update non-conflicting submodules"
        new_submods=$(git submodule status | grep -E '^\+' | awk '{print $2}')
        for mod in $new_submods; do
            git add "$mod"
        done
        git commit -s -m 'Update released submodules'
    fi

    verify_clean_workarea "$indent"

    update_nnf_mfu_release_references "$repo_short_name" "$indent"
    verify_clean_workarea "$indent"

    if [[ $repo_short_name == nnf_deploy ]]; then
        update_remote_release_references "$indent"
        verify_clean_workarea "$indent"

        # Tidy and make nnf-deploy to avoid embarrassment later.
        go mod tidy
        if [[ $(git status -s | wc -l) -gt 0 ]]; then
            git add go.mod go.sum
            git commit -s -m 'go mod tidy'
        fi
        verify_clean_workarea "$indent"
        make || do_fail "${indent}Failure building the nnf-deploy binary."

    elif [[ $repo_short_name == nnf_doc ]]; then
        update_nnf_release_reference "$indent"
        verify_clean_workarea "$indent"
    fi

    if [[ $(git log --oneline "$branch...HEAD" | wc -l) -gt 0 ]]; then
        # We have updates, so let's designate a new release.
        update_own_release_references "$repo_short_name" "$new_release" "$indent"
        has_changes=true
    else
        msg "${indent}No new changes to release for $repo_name."
        cd ..
        return
    fi

    verify_clean_workarea "$indent"

    echo
    msg "${indent}Commits added to branch $repo_name/$new_branch:"
    git log --oneline "$branch...HEAD" | cat
    echo

    if [[ $has_changes = "true" && $PUSH_BRANCH = "true" ]]; then
        git push --set-upstream origin "$new_branch" || do_fail "${indent}Failure pushing new release branch."
    fi

    echo
    cd ..
}

create_pr_release_vX() {
    local repo_name="$1"
    local repo_url="$2"
    local branch="$3"
    local indent="  "

    msg "Repo $repo_name/$branch:"
    begin_with_clean_workarea "$repo_name" "$indent"

    clone_checkout_fresh_workarea "$repo_name" "$repo_url" "$branch" "$indent"

    latest_release=$(git describe --match="v*" --abbrev=0 HEAD) || do_fail "${indent}Failure getting latest release tag."
    new_release=$(semver_bump "$latest_release")

    verify_clean_workarea "$indent"

    new_branch="release-$new_release"
    if ! git checkout "$new_branch"; then
        echo
        msg "${indent}Failure checking out branch $repo_name/$new_branch. Was it pushed?"
        echo
        exit 1
    fi

    if [[ -f .gitmodules ]]; then
        git submodule update
    fi

    verify_clean_workarea "$indent"

    gh pr create --base "$branch" --head "$new_branch" --title "Release $new_release" --body "Release $new_release"

    echo
    cd ..
}

merge_pr_release_vX() {
    local repo_name="$1"
    local repo_url="$2"
    local branch="$3"
    local indent="  "

    msg "Repo $repo_name/$branch:"
    begin_with_clean_workarea "$repo_name" "$indent"

    clone_checkout_fresh_workarea "$repo_name" "$repo_url" "$branch" "$indent"

    latest_release=$(git describe --match="v*" --abbrev=0 HEAD) || do_fail "${indent}Failure getting latest release tag."
    new_release=$(semver_bump "$latest_release")
    new_branch="release-$new_release"

    echo
    msg "${indent}New release will be: $new_release"
    echo

    msg "${indent}Checking for existing pushed branch $repo_name/$new_branch."
    gh pr checkout "$new_branch" || do_fail "${indent}Failure checking out PR branch $new_branch"
    gh pr merge --merge --delete-branch --subject "Merge release $new_release" || do_fail "${indent}Failure merging PR."

    echo
    cd ..
}

tag_release_vX() {
    local repo_short_name="$1"
    local repo_name="$2"
    local repo_url="$3"
    local branch="$4"
    local indent="  "

    msg "Repo $repo_name/$branch:"
    begin_with_clean_workarea "$repo_name" "$indent"

    clone_checkout_fresh_workarea "$repo_name" "$repo_url" "$branch" "$indent"

    latest_release=$(git describe --match="v*" --abbrev=0 HEAD) || do_fail "${indent}Failure getting latest release tag."

    most_recent_commit=$(git log --oneline -1)
    if [[ "$most_recent_commit" =~ " Merge release " ]]; then
        merge_release=$(git log --oneline -1 | sed 's/^.* Merge release \(.*\)/\1/')
    elif [[ -n $FORCE_TAG_VALUE ]]; then
        echo
        msg "${indent}WARNING"
        msg "${indent}Using -x override to set release version: $FORCE_TAG_VALUE"
        msg "${indent}WARNING"
        echo
        merge_release="$FORCE_TAG_VALUE"
    fi
    if [[ -z $merge_release ]]; then
        do_fail "${indent}Did not find the merge commit, or a -x override."
    fi
    msg "${indent}Expecting to tag as release $merge_release"

    # Is it already tagged?
    if git show "$merge_release" 2>/dev/null 1>&2; then
        msg "${indent}Already tagged as $merge_release"
    else
        msg "${indent}Tagging as $merge_release"
        git tag -a "$merge_release" -m "Release $merge_release" || do_fail "${indent}Failed tagging as $merge_release"
        git push origin --tags || do_fail "${indent}Failed to push tags"
    fi

    if [[ $repo_short_name == nnf_doc ]]; then
        # Is the doc's release already created?
        if gh release view "$merge_release" > /dev/null 2>&1; then
            msg "${indent}Already created release doc"
        else
            msg "${indent}Creating release doc."
            msg "${indent}Generating notes from $latest_release to $merge_release."
            gh release create --generate-notes --verify-tag --notes-start-tag "$latest_release" "$merge_release" || do_fail "${indent}Failed to generate release for $repo_short_name"
        fi
    fi

    echo
    cd ..
}

check_if_wants_only_one() {
    local repo_count
    local phase=$PHASE

    repo_count=$(echo "$REPO_LIST" | wc -w | awk '{print $1}')
    if [[ $repo_count -gt 1 && -n $ALLOW_VENDOR_MULTI_API ]]; then
        do_fail "Use of -M requires -R with only one repo specified."
    fi
    if [[ $repo_count -gt 1 ]]; then
        local fail=false
        case $phase in
        master) ;;
        release)
            if [[ -n $PUSH_BRANCH ]]; then
                fail=true
                phase="release-push"
            fi
            ;;
        *) fail=true ;;
        esac

        if [[ $fail == "true" ]]; then
            do_fail "Phase $phase requires -R with only one repo specified."
        fi
    fi
}

run_phase() {
    check_if_wants_only_one
    for repo_short_name in $REPO_LIST; do
        url=$(getter repomap "$repo_short_name")
        name=$(get_repo_dir_name "$url")
        echo "===================="

        case $PHASE in
        master)
            check_repo_master "$repo_short_name" "$name" "$url"
            ;;
        release)
            check_repo_release_vX "$repo_short_name" "$name" "$url" "releases/v0"
            ;;
        create_pr)
            create_pr_release_vX "$name" "$url" "releases/v0"
            ;;
        merge_pr)
            merge_pr_release_vX "$name" "$url" "releases/v0"
            ;;
        tag_release)
            tag_release_vX "$repo_short_name" "$name" "$url" "releases/v0"
            ;;
        *)
            msg "Phase '$PHASE' not yet implemented."
            ;;
        esac
    done
}

run_phase

