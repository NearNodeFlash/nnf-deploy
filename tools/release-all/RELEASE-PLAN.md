# NNF Software Release Plan

This document is the authoritative, self-contained guide for executing an NNF software release. All release phases are driven by `release-all.sh` in this directory, wrapped by the `nnf_cmd` helper function. The release process is the same regardless of how many repos have changes — the `release` phase automatically detects which repos have new commits since the last release and skips those that don't. An agent or human following this document should be able to execute the entire release without external context.

---

## Repository Reference

| Script Name | GitHub Repo | Owner | Default Branch | Notes |
| --- | --- | --- | --- | --- |
| `dws` | `DataWorkflowServices/dws` | DataWorkflowServices | `master` | |
| `lustre_csi_driver` | `HewlettPackard/lustre-csi-driver` | HewlettPackard | `master` | |
| `lustre_fs_operator` | `NearNodeFlash/lustre-fs-operator` | NearNodeFlash | `master` | |
| `nnf_mfu` | `NearNodeFlash/nnf-mfu` | NearNodeFlash | `master` | Standalone, not a submodule |
| `nnf_ec` | `NearNodeFlash/nnf-ec` | NearNodeFlash | `master` | Standalone, not a submodule |
| `nnf_storedversions_maint` | `NearNodeFlash/nnf-storedversions-maint` | NearNodeFlash | **`main`** (not master) | Standalone, not a submodule |
| `nnf_sos` | `NearNodeFlash/nnf-sos` | NearNodeFlash | `master` | Requires `-M` flag |
| `nnf_dm` | `NearNodeFlash/nnf-dm` | NearNodeFlash | `master` | Vendors nnf-sos |
| `nnf_integration_test` | `NearNodeFlash/nnf-integration-test` | NearNodeFlash | `master` | Vendors nnf-sos |
| `nnf_deploy` | `NearNodeFlash/nnf-deploy` | NearNodeFlash | `master` | Has submodules for most repos |
| `nnf_doc` | `NearNodeFlash/NearNodeFlash.github.io` | NearNodeFlash | **`main`** (not master) | Released last; tracks nnf-deploy version |

**PR Reviewers:** Defined in `.github/CODEOWNERS`. Assign all code owners **except** the person creating the release. The `setup-release-env.sh` helper reads CODEOWNERS automatically and sets `$RELEASE_REVIEWERS`.

**Dependency chain (release order):**

```text
dws → lustre_csi_driver → lustre_fs_operator → nnf_mfu → nnf_ec → nnf_storedversions_maint → nnf_sos → nnf_dm → nnf_integration_test → nnf_deploy → nnf_doc
```

**Vendoring dependencies (peer modules in `go.mod`):**

- `lustre_fs_operator` vendors `dws`
- `nnf_sos` vendors `dws`, `lustre-fs-operator`, `nnf-ec`
- `nnf_dm` vendors `dws`, `lustre-fs-operator`, `nnf-ec` (indirect), `nnf-sos`
- `nnf_integration_test` vendors `dws`, `lustre-fs-operator`, `nnf-ec` (indirect), `nnf-sos`
- `nnf_storedversions_maint` has **no** NNF peer vendoring dependencies

If an upstream repo changes, **all downstream repos that vendor it** may need revendoring before the release (see Step 2a). The vendoring check in Step 2 catches these automatically.

---

## Release Execution Plan

### Prerequisites (Pre-flight Checklist)

1. Ensure these tools are installed: `gh`, `yq` (Go version v4.x), `jq`, `perl`, `git`, `make`, `tput`, `sed`
2. Set `GH_TOKEN` env var with a GitHub **classic** token (not fine-grained) with `repo` scope. The token is 40 characters, starting with `ghp_`.
3. Verify `gh` authentication: `gh auth status`. Confirm it shows the expected user and token scopes.
4. Verify SSH access to all repo URLs (`ssh -T git@github.com`)
5. Decide the release type (`-B major|minor|patch`). **Always pass `-B` explicitly** — do not rely on the script default, which could change. Most releases use `-B patch`.
6. **Pre-release vendoring check:** If any upstream repo has had changes merged to master since the last release that affect downstream vendoring, ensure downstream repos have been revendored **before** starting the release. For example, if `nnf-sos` changed, then `nnf-dm` and `nnf-integration-test` (which vendor `nnf-sos`) must have their vendor directories updated via PRs merged to master. Phase 1's vendoring checks will catch any mismatches.

### Phase 0: Fresh Clone

> The release uses a **volatile** working directory that is deleted and recreated at the start of each release. Set `$RELEASE_WORKDIR` to the desired location (e.g., `RELEASE_WORKDIR=~/release-work`).

#### Step 0 — Clone nnf-deploy with submodules

If `$RELEASE_WORKDIR` already exists, confirm with the user that it can be deleted. **If they say no, halt the release process.**

```sh
# Set the working directory for this release
export RELEASE_WORKDIR=~/release-work   # adjust as needed

# If $RELEASE_WORKDIR exists, ask the user before proceeding.
# If they decline, stop here.

# Clean build artifacts before removing (rm -rf can fail on build outputs)
if [ -d "$RELEASE_WORKDIR/nnf-deploy/workingspace" ]; then
    for repo in "$RELEASE_WORKDIR"/nnf-deploy/workingspace/*/; do
        ( cd "$repo" && make clean-bin 2>/dev/null || true )
    done
fi

rm -rf "$RELEASE_WORKDIR"
mkdir -p "$RELEASE_WORKDIR"
cd "$RELEASE_WORKDIR"
git clone --recurse-submodules git@github.com:NearNodeFlash/nnf-deploy.git
cd nnf-deploy
```

All subsequent steps run from `$RELEASE_WORKDIR/nnf-deploy`.

### General Notes

> **Helper functions:** After sourcing `setup-release-env.sh`, use `nnf_cmd <phase> <repo>` for all `release-all.sh` invocations. It automatically handles the `-B` flag, the `-M` flag for `nnf_sos`, and pipes through `cat` to avoid pager issues. Use `nnf_create_pr <repo>` for the `create-pr` phase (captures `$PR_NUMBER`) and `nnf_add_reviewers <repo>` to assign reviewers.
>
> **Manual invocations:** If you need to call `release-all.sh` directly, always pass `-B patch` (or `-B minor`/`-B major`) explicitly, add `-M` for `nnf_sos`, and append `2>&1 | cat`.

### Phase 1: Discovery & Validation

#### Step 1 — Set up and list repos

```sh
cd tools/release-all
```

Source the release environment helper to auto-compute versions, detect the release operator, and set reviewers:

```sh
source ./setup-release-env.sh -B patch   # or -B minor / -B major
```

> **Important:** Never pipe this `source` command (e.g. `source ... | cat`). Piping runs `source` in a subshell, which means the exported variables and function definitions (`nnf_cmd`, etc.) are lost to the calling shell. If you need to capture output, redirect to a file or run without the pipe.

This exports `$PREVIOUS_RELEASE`, `$NNF_RELEASE`, `$RELEASE_TYPE`, and `$RELEASE_REVIEWERS`, and defines helper functions `nnf_cmd`, `nnf_create_pr`, `nnf_add_reviewers`, and `nnf_gh_repo`. Confirm the printed summary with the user before proceeding.

List repos:

```sh
./release-all.sh -L
```

Save the ordered list: `dws`, `lustre_csi_driver`, `lustre_fs_operator`, `nnf_mfu`, `nnf_ec`, `nnf_storedversions_maint`, `nnf_sos`, `nnf_dm`, `nnf_integration_test`, `nnf_deploy`, `nnf_doc`.

#### Step 2 — Check vendoring *(sequential, per repo)*

> Must be error-free before proceeding. This runs on **all 10 repos** including `nnf_doc` — vendoring checks validate the current state of master/main, which is independent of the release branching.

```sh
for repo in $(./release-all.sh -L); do
    nnf_cmd master "$repo"
done
```

> **Important:** Run vendoring checks on **all** repos, not just the ones you plan to release. This catches stale submodule pointers in `nnf-deploy` (e.g., a force-push on a submodule repo can leave the pointer at an orphaned commit SHA).

#### Step 2a — Fix stale vendoring *(if Step 2 fails)*

If the vendoring check for a repo reports **"Peer modules are behind"**, the error output includes the exact `go get` commands needed to update the vendor. Follow this procedure for each failing repo, working **in dependency order** per the vendoring table above (e.g., fix `lustre_fs_operator` before `nnf_sos`, fix `nnf_sos` before `nnf_dm` or `nnf_integration_test`).

For each failing repo:

1. **Read the error output.** It contains the `go get` command(s) with the specific module paths and branch targets.
2. **Clone the repo** into a temporary directory using `nnf_gh_repo` to resolve the GitHub `owner/repo` path.
3. **Create a branch** named `update-vendor`.
4. **Run the `go get` commands** from the error output, then run `go mod tidy` and `go mod vendor`.
5. **Commit** all changes with a signed-off commit (`-s`) and message `"Update vendor dependencies"`.
6. **Push** the branch and **create a PR** titled `"Update vendor dependencies"` with body `"Pre-release vendoring update."`.
7. **Assign reviewers** from `$RELEASE_REVIEWERS`.
8. **Wait for CI** and reviewer approval, then **merge** the PR.
9. **Verify the fix against origin:** Re-clone (repeat Step 0) so you're working from what's actually on GitHub, then re-run the `nnf_cmd master <repo>` check starting from the repo you just fixed and continuing through all remaining downstream repos in order. The fixed repo must now pass. If a downstream repo fails, repeat this procedure (steps 1–9) for it.
10. Return to `$RELEASE_WORKDIR/nnf-deploy/tools/release-all`.

> **Dependency order matters:** An upstream repo's vendoring PR must be merged to master before any downstream repo that vendors it can be fixed. For example, if both `nnf_sos` and `nnf_dm` fail, merge the `nnf_sos` fix first — `nnf_dm` vendors `nnf_sos` and needs the updated master.
>
> **`nnf_deploy` submodule pointers:** Any submodule repo that has had commits merged to master since the last submodule pointer update — whether from development work or vendoring fixes — will cause `nnf_deploy`'s Step 2 check to fail with "Submodules are not up to date." The script catches this automatically. Fix it the same way: clone `nnf_deploy`, create a branch, run `git submodule update --remote` for the affected submodules, commit, PR, merge. Then re-clone and verify.

After fixing and verifying the last failing repo, **re-run Step 2 on all repos** as a final safety net to confirm everything passes end to end.

### Phase 2: Branch Creation

#### Step 3 — Create trial release branches *(sequential, per repo)*

Process all repos **except `nnf_doc`**, which is deferred (see below):

```sh
for repo in $(./release-all.sh -L | grep -v nnf_doc); do
    nnf_cmd release "$repo"
done
```

Review output for merge conflicts before continuing.

> **Note:** `nnf_mfu`, `nnf_ec`, and `nnf_storedversions_maint` are standalone repos that often have no new commits between releases. If any reports "No new changes to release", this is normal — **skip that repo in all subsequent phases** (Steps 4a–4d and the `nnf_doc` sequence). Do not attempt to push, PR, merge, or tag a repo that had no changes.

#### `nnf_doc` deferral

`nnf_doc` must be deferred until after `nnf_deploy` is tagged and its GitHub Release is published. The `nnf_doc` script updates `mkdocs.yml` with the latest `nnf-deploy` release version by querying GitHub Releases (not just tags). Follow this sequence:

1. Complete Steps 3–4d for all repos through `nnf_deploy`
2. **Wait ~60 seconds** for `nnf_deploy`'s "Handle Release Tag" GitHub Actions workflow to publish the GitHub Release
3. Verify: `gh release view $NNF_RELEASE -R NearNodeFlash/nnf-deploy 2>&1 | cat` should succeed. If it fails with "release not found", wait another 30 seconds and retry. The workflow typically completes within 2 minutes; if it hasn't after 5 minutes, check the Actions tab on the `nnf-deploy` repo for errors.
4. Then run `nnf_doc` through Steps 3–4d:

```sh
nnf_cmd release nnf_doc
nnf_cmd release-push nnf_doc
nnf_create_pr nnf_doc
nnf_add_reviewers nnf_doc
nnf_cmd merge-pr nnf_doc
nnf_cmd tag-release nnf_doc
```

> **Important:** `nnf_doc`'s repo is `NearNodeFlash/NearNodeFlash.github.io` and uses the `main` branch (not `master`).

### Phase 3: Release Generation

*Complete steps 4a–4d for each repo sequentially before moving on to the next. Present the results of all four sub-steps to the user and wait for approval once per repo (after step 4d), rather than after each individual sub-step.*

> **Why sequential order matters:** Phase 2 creates release branches (not tags). Phase 3 then processes each repo through push → PR → merge → tag **in dependency order**. By the time a downstream repo (e.g., `nnf_dm`) reaches `create-pr`, any upstream repo it depends on (e.g., `nnf_sos`) is already tagged. The script's `release` phase handles submodule pointers to the correct release branches automatically.

#### Step 4a — Push release branch

*Only if no merge conflicts.*

```sh
nnf_cmd release-push <repo>
```

#### Step 4a-alt — Merge conflict resolution

*Only if merge conflicts exist.* **Stop and present the conflict details to the user.** Merge conflicts during a release are rare and require human judgment. Do not attempt to resolve them automatically. Wait for the user to provide guidance before proceeding.

#### Step 4b — Create PR

```sh
nnf_create_pr <repo>
```

The `create-pr` output includes the PR URL (e.g., `https://github.com/<owner>/<repo>/pull/123`). The `nnf_create_pr` function extracts the PR number and sets `$PR_NUMBER` automatically.

> **Note:** The script does **not** assign PR reviewers automatically. After each `create-pr`, assign reviewers:

```sh
nnf_add_reviewers <repo>
```

#### Step 4c — Merge PR

> Do NOT manually merge — let the tool do it.

```sh
nnf_cmd merge-pr <repo>
```

> **Note:** Before running `merge-pr`, ensure CI status checks on the PR have passed. If required checks are still running, `merge-pr` may fail.
>
> **Note:** `merge-pr` may produce no visible output even on success. Verify the merge completed:
>
> ```sh
> gh api repos/<owner>/<repo>/pulls/<pr_number> 2>&1 | grep -o '"merged":[^,]*'
> ```
>
> Expected output: `"merged":true`

#### Step 4d — Tag the release

*Creates the annotated tag required by CI/CD.*

```sh
nnf_cmd tag-release <repo>
```

> **Note:** `tag-release` output will include "Bypassed rule violations for refs/tags/...". This is expected. The repos have rulesets ("Auto-imported tag create protections" and "Auto-imported tag delete protections") that block creation, update, and deletion of `v*` tags by default — protecting release tags from unauthorized changes. Your account bypasses these rules because it has an admin or maintain role, which is in the ruleset's bypass list. The tags are created correctly.

### Phase 4: Finalize

#### Step 5 — Finalize release notes

*Run only after ALL repos, including `nnf_doc`, are released. Run from `tools/release-all/`.*

```sh
./final-release-notes.sh -r $NNF_RELEASE 2>&1 | cat        # preview
./final-release-notes.sh -r $NNF_RELEASE -C 2>&1 | cat      # commit
```

### Phase 5: Verification

#### Step 6 — Compare release manifests

*Run from `tools/release-all/`.*

```sh
./compare-releases.sh -i $PREVIOUS_RELEASE $NNF_RELEASE 2>&1 | cat
```

The `-i` flag displays image version changes inline. Use `-d` to display the full diff. The diff file is always saved to `workingspace/manifest-<ver1>-to-<ver2>.diff`.

#### Step 7 — Verify GitHub releases

Check each repo for correct tag, release notes, and artifacts (`manifests.tar` + `manifests-kind.tar` on `nnf-deploy`).

---

### Relevant files

- `tools/release-all/release-all.sh` — Main orchestration script, all phases
- `tools/release-all/final-release-notes.sh` — Release notes finalization
- `tools/release-all/compare-releases.sh` — Manifest diff tool
- `tools/release-all/README.md` — Minimal, needs expansion
- `.github/workflows/handle_release_tag.yaml` — CI/CD that creates GitHub releases on tag push
- `config/repositories.yaml` — Submodule version tracking updated by release-push phase
- External docs source: NearNodeFlash/NearNodeFlash.github.io repo

### Decisions

- Release execution follows the strict 10-repo dependency order — no parallelization
- `nnf-mfu` and `nnf-ec` are released standalone (positions 4 & 5) despite not being submodules
- `nnf_doc` released last, version should match nnf-deploy
- Default `-B patch` unless breaking changes present

### Verification

1. `./release-all.sh -L` confirms 10 repos in dependency order
2. Zero errors from each `master` phase before proceeding
3. GitHub release page shows correct annotated tag after each `tag-release`
4. `compare-releases.sh` diff validates expected changes
5. `final-release-notes.sh` output includes all submodule notes + CRD API info
6. nnf-deploy GitHub release has manifests.tar and manifests-kind.tar

### Further Considerations

1. **Release type** — Is this patch, minor, or major? Determines `-B` flag. Recommend: default `patch` unless breaking changes.
2. **Doc improvements timing** — Corrections applied first (branch `improve-release-docs` in NearNodeFlash.github.io), to be used as a guide during the release. Enhancements can follow after the release.
3. **README expansion** — Add prerequisites and quick-start to `tools/release-all/README.md` inline; keep detailed guide external.
