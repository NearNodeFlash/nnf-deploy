#!/usr/bin/env python3

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

"""Vendor the specified CRD version, updating Go code to point at it."""

import argparse
import os
import sys
import yaml

from pkg.git_cli import GitCLI
from pkg.make_cmd import MakeCmd
from pkg.vendoring import Vendor
from pkg.go_cli import GoCLI

WORKING_DIR = "workingspace"

PARSER = argparse.ArgumentParser()
PARSER.add_argument(
    "--hub-ver",
    type=str,
    required=True,
    help="Version of the hub API.",
)
PARSER.add_argument(
    "--module",
    "-m",
    type=str,
    required=True,
    help="Go module which has the versioned API, specified the way it is found in go.mod.",
)
PARSER.add_argument(
    "--repo",
    "-r",
    type=str,
    required=True,
    help="Git repository URL which has the Go code that consumes the APIs.",
)
PARSER.add_argument(
    "--branch",
    "-b",
    type=str,
    required=False,
    help="Branch name to create. Default is 'api-<repo-name>-<new-ver>'",
)
PARSER.add_argument(
    "--this-branch",
    action="store_true",
    dest="this_branch",
    help="Continue working in the current branch. Use when stepping through with 'step'.",
)
PARSER.add_argument(
    "--dry-run",
    "-n",
    action="store_true",
    dest="dryrun",
    help="Dry run. Implies only one step.",
)
PARSER.add_argument(
    "--no-commit",
    "-C",
    action="store_true",
    dest="nocommit",
    help="Skip git-commit. Implies only one step.",
)
PARSER.add_argument(
    "--workdir",
    type=str,
    required=False,
    default=WORKING_DIR,
    help=f"Name for working directory. All repos will be cloned below this directory. Default: {WORKING_DIR}.",
)


def main():
    """main"""

    args = PARSER.parse_args()

    gitcli = GitCLI(args.dryrun, args.nocommit)
    gitcli.clone_and_cd(args.repo, args.workdir)

    gocli = GoCLI(args.dryrun)

    # Load any repo-specific local config.
    bumper_cfg = None
    if os.path.isfile("crd-bumper.yaml"):
        with open("crd-bumper.yaml", "r", encoding="utf-8") as fi:
            bumper_cfg = yaml.safe_load(fi)

    makecmd = MakeCmd(args.dryrun, None, None, None)

    if args.branch is None:
        bn = os.path.basename(args.module)
        args.branch = f"api-{bn}-{args.hub_ver}"
    if args.this_branch:
        print("Continuing work in current branch")
    else:
        print(f"Creating branch {args.branch}")
        try:
            gitcli.checkout_branch(args.branch)
        except RuntimeError as ex:
            print(str(ex))
            print(
                "If you are continuing in an existing branch, then specify `--this-branch`."
            )
            sys.exit(1)

    vendor_new_api(args, makecmd, gitcli, gocli, bumper_cfg)


def vendor_new_api(args, makecmd, git, gocli, bumper_cfg):
    """Vendor the new API into the repo."""

    vendor = Vendor(args.dryrun, args.module, args.hub_ver)

    if vendor.uses_module() is False:
        print(
            f"Module {args.module} is not found in go.mod in {args.repo}. Nothing to do."
        )
        sys.exit(0)
    try:
        vendor.set_current_api_version()
        vendor.set_preferred_api_alias()
    except ValueError as ex:
        print(str(ex))
        sys.exit(1)

    print(f"Updating files from {vendor.current_api_version()} to {args.hub_ver}")

    # Update the Go files that are in the usual kubebuilder locations.
    vendor.update_go_files()

    # Bump any other, non-controller, directories of code.
    if bumper_cfg is not None and "extra_go_dirs" in bumper_cfg:
        for extra_dir in bumper_cfg["extra_go_dirs"].split(","):
            vendor.update_go_files(extra_dir)
    # Bump any necessary references in the config/ dir.
    if bumper_cfg is not None and "extra_config_dirs" in bumper_cfg:
        for extra_dir in bumper_cfg["extra_config_dirs"].split(","):
            vendor.update_config_files(extra_dir)

    gocli.get(args.module, "master")
    gocli.tidy()
    gocli.vendor()
    vendor.verify_one_api_version()

    makecmd.manifests()
    makecmd.generate()
    makecmd.generate_go_conversions()
    makecmd.fmt()
    makecmd.clean_bin()

    vendor.commit(git, "vendor-new-api")


if __name__ == "__main__":
    main()

sys.exit(0)
