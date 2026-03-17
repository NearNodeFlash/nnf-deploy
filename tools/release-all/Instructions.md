# Agent Instructions for NNF Software Release

> **Agent entry point:** In VS Code, invoke the `@release` agent from the chat picker (defined in `.github/agents/release.agent.md`). The agent automatically loads this file and `RELEASE-PLAN.md` as context.

## Overview

Execute the release plan defined in `RELEASE-PLAN.md` (in this same directory). That document is the authoritative, self-contained reference for the release process. Read it completely before starting.

## Goal

Create a release of the NNF software while ensuring the release process documented in `RELEASE-PLAN.md` is correct and repeatable. If you discover gaps, errors, or missing details in the plan, update it.

## Agent Behavior

### Execution Model

- Run all commands in a terminal window so the user can see them.
- Append `2>&1 | cat` to all `release-all.sh` and `gh` commands to avoid pager issues.
- Execute one phase per repo at a time. Do not parallelize across repos.
- Complete steps 4a–4d for each repo before moving to the next.

### Approval Gates

- **Wait for the user to approve** after each step before proceeding.
- After each step, clearly state:
  1. What you did
  2. Whether you believe it succeeded
  3. The evidence (command output, exit codes, API responses)
- The user will respond with "approved", "approved. continue", or will coach you if something is wrong.

### Error Handling

- If a command fails, do **not** retry blindly. Analyze the error, explain it, and propose a fix.
- If `merge-pr` produces no output, verify via: `gh api repos/<owner>/<repo>/pulls/<pr_number> 2>&1 | grep -o '"merged":[^,]*'`
- If `tag-release` shows "Bypassed rule violations for refs/tags/..." — this is expected (tag protection rulesets allow admin/maintain bypass).
- If a vendoring check fails, investigate the submodule pointer or vendor directory before proceeding.
- Never use `--force`, `--no-verify`, or destructive operations without asking the user first.

### Release Process

The release process is the same regardless of how many repos have changes. The `release` phase automatically detects which repos have new commits since the last release. Repos with no changes report "No new changes to release" and are skipped in subsequent phases. Simply run all phases on all repos — the script handles the rest.

### Key Gotchas (Learned from Experience)

- `nnf_sos` requires the `-M` flag on ALL phases (not just `master`).
- `nnf_mfu` and `nnf_ec` often have no changes — "No new changes to release" is normal, skip them.
- `nnf_doc` must wait until after `nnf_deploy`'s GitHub Release is published (~60s after tagging). Verify with: `gh release view $NNF_RELEASE -R NearNodeFlash/nnf-deploy`
- `nnf_doc` uses the `main` branch (not `master`). Its repo is `NearNodeFlash/NearNodeFlash.github.io`.
- When a dependency changes (e.g., `nnf-sos`), all downstream repos that vendor it (`nnf-dm`, `nnf-integration-test`) must be revendored and their PRs merged **before** starting the release. If the vendoring check (Step 2) fails with "Peer modules are behind", follow Step 2a in RELEASE-PLAN.md to fix it. The check output prints the exact `go get` commands needed.
- Always run vendoring checks on ALL repos — this catches stale submodule pointers.
- After completing all releases, finalize release notes with `final-release-notes.sh` and run `compare-releases.sh` for verification.

### Updating the Plan

If you encounter an issue not covered by RELEASE-PLAN.md, or if a step's success criteria are unclear:

1. Resolve the issue with the user's guidance.
2. Update RELEASE-PLAN.md with what you learned.
3. Note the update to the user.

## Quick Start

```sh
# 1. Read the plan
cat RELEASE-PLAN.md

# 2. Ask the user: release type? (patch/minor/major, default patch)

# 3. Source the helper: source ./setup-release-env.sh -B patch
#    Confirm the printed summary with the user

# 4. Follow RELEASE-PLAN.md step by step, waiting for approval at each gate.
```
