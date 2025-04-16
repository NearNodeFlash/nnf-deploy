#!/usr/bin/env python3

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

"""Vendor the specified CRD version, updating Go code to point at it."""

import argparse
import os
import sys
import yaml

from pkg.git_cli import GitCLI
from pkg.make_cmd import MakeCmd
from pkg.vendoring import Vendor
from pkg.go_cli import GoCLI
from pkg.hub_spoke_util import HubSpokeUtil

WORKING_DIR = "workingspace"

PARSER = argparse.ArgumentParser()
PARSER.add_argument(
    "--hub-ver",
    type=str,
    required=False,
    help="Version of the hub API of the target repo.",
)
PARSER.add_argument(
    "--vendor-hub-ver",
    type=str,
    required=True,
    help="Version of the hub API for the repo being vendored.",
)
PARSER.add_argument(
    "--module",
    "-m",
    type=str,
    required=True,
    help="Go module which has the versioned API, specified the way it is found in go.mod.",
)
PARSER.add_argument(
    "--version",
    "-v",
    type=str,
    required=False,
    default="master",
    help="Version for the go module which has the versioned API",
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
    help="Continue working in the current branch.",
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
PARSER.add_argument(
    "-M",
    action="store_true",
    dest="multi_vend",
    help="Allow multiple API versions to be vendored from a single peer module. Expect this to be an unusual case.",
)


def main():
    """main"""

    args = PARSER.parse_args()

    gitcli = GitCLI(args.dryrun, args.nocommit)
    gitcli.clone_and_cd(args.repo, args.workdir)

    gocli = GoCLI(args.dryrun)

    if args.hub_ver:
        if not HubSpokeUtil.is_hub(args.hub_ver):
            print(f"API --hub-ver {args.hub_ver} is not a hub.")
            sys.exit(1)

    # Load any repo-specific local config.
    bumper_cfg = {}
    if os.path.isfile("crd-bumper.yaml"):
        with open("crd-bumper.yaml", "r", encoding="utf-8") as fi:
            bumper_cfg = yaml.safe_load(fi)

    makecmd = MakeCmd(args.dryrun, None, None, None)

    if args.branch is None:
        bn = os.path.basename(args.module)
        args.branch = f"api-{bn}-{args.vendor_hub_ver}"
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

    vendor = Vendor(
        args.dryrun, args.module, args.hub_ver, args.vendor_hub_ver, args.multi_vend
    )

    if vendor.uses_module() is False:
        print(
            f"Module {args.module} is not found in go.mod in {args.repo}. Nothing to do."
        )
        sys.exit(0)
    try:
        if not os.path.isdir("vendor"):
            # This repo doesn't normally vendor, so go get it.
            # The .gitignore file should already be covering it.
            gocli.tidy()
            gocli.vendor()
        vendor.set_current_api_version()
        alt_main_file = None
        if "alternate_main" in bumper_cfg:
            alt_main_file = bumper_cfg["alternate_main"]
        vendor.set_preferred_api_alias(alt_main_file)
    except ValueError as ex:
        print(str(ex))
        sys.exit(1)

    print(
        f"Updating files from {vendor.current_api_version()} to {args.vendor_hub_ver}"
    )

    # Update the Go files that are in the usual kubebuilder locations.
    vendor.update_go_files()

    # Bump any other, non-controller, directories of code.
    if "extra_go_dirs" in bumper_cfg:
        for extra_dir in bumper_cfg["extra_go_dirs"].split(","):
            vendor.update_go_files(extra_dir)
    if "extra_go_files" in bumper_cfg:
        for full_path in bumper_cfg["extra_go_files"].split(","):
            vendor.update_go_file(full_path)
    # Bump any necessary references in the config/ dir.
    if "extra_config_dirs" in bumper_cfg:
        for extra_dir in bumper_cfg["extra_config_dirs"].split(","):
            vendor.update_config_files(extra_dir)

    gocli.get(args.module, args.version)
    gocli.tidy()
    gocli.vendor()

    have_controller_gen = True
    if "skip_controller_gen" in bumper_cfg:
        if bumper_cfg["skip_controller_gen"] is True:
            have_controller_gen = False

    if have_controller_gen:
        makecmd.manifests()
        makecmd.generate()
        makecmd.generate_go_conversions()
    makecmd.fmt()
    if have_controller_gen:
        makecmd.clean_bin()

    vendor.commit(git, "vendor-new-api")


if __name__ == "__main__":
    main()

sys.exit(0)
