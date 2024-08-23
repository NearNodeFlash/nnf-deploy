# release_all.sh

- [Overview](#overview)
- [Assumptions](#assumptions)
- [Steps](#steps)

## Overview

`release-all.sh` automates most of the steps of [releasing NNF software](https://nearnodeflash.github.io/latest/repo-guides/release-nnf-sw/readme/) and adds additional checks for common issues. Please visit [releasing NNF software](https://nearnodeflash.github.io/latest/repo-guides/release-nnf-sw/readme/) for background information about the structure of the NNF Software.

## Assumptions

- `master/main` branch for each repository contains **tested** software and documentation ready to be released.
- You've installed the GitHub CLI tool, `gh`.
  - This tool requires a GH_TOKEN environment variable containing a `repo` scope classic token.

## Steps

### Run the steps in this order

> **Note:** You almost always want to use the -R option to focus the `phase` activity to a specific repo.

0. **List Repos:** Get the ordered list of repo names to use with -R option in subsequent steps. This is referred to as `repo-list`
    > **Pro tip** Keep this list in a separate window for easy viewing
    ./release-all.sh -L

1. **Check Vendoring:** For each repo's master/main branch; determine whether any of them need to be re-vendored.
    > **Note:** Ensure each repo is error-free before proceeding to the next repo in `repo-list`

    ```bash
    For each repo in `repo-list`
        ./release-all.sh -P master -R $repo
    ```

2. **Create Trial Release Branch:** Create the new release branch, merge master/main to that release branch, but don't push it yet. The point of this step is to look for merge conflicts between master/main and the release branch.

    ```bash
    For each repo in `repo-list`
        ./release-all.sh -P release -R $repo
    ```

3. **Generate Release:** For each repo in `repo-list`, proceed through the following steps in sequence before moving on to the next repo.
    > **Note:** The next steps use the gh(1) GitHub CLI tool and require a GH_TOKEN environment variable containing a 'repo' scope classic token.

    1. If the **Create Trial Release Branch** had no errors
      ./release-all.sh -P release-push -R <repo>

        Else if **Create Trial Release Branch** was unable to auto merge, manually merge and push the release branch

        ```bash
        cd workingspace/repo
        # Manually merge the changes from master/main to the release branch
        go mod tidy
        go mod vendor
        git status # confirm all issues have been address
        git add <all affected files>
        git commit -s # take the default commit message, don't bother editing it.
        git push --set-upstream origin <release branch name>
        ```

    2. Create PR for the pushed release branch:
      ./release-all.sh -P create-pr -R <repo>
    3. Merge PR for the pushed release branch: **Note: Do NOT manually merge the PR, let `release-all.sh` do it**
      ./release-all.sh -P merge-pr -R <repo>
    4. Tag the release:
      ./release-all.sh -P tag-release -R <repo>
