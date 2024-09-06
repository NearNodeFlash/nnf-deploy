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

"""Bump the CRD version, adding conversion webhooks and tests."""

import argparse
import os
import sys

from pkg.webhooks import MvWebhooks, ConversionWebhooks
from pkg.project import Project
from pkg.create_apis import CreateApis
from pkg.controllers import Controllers
from pkg.git_cli import GitCLI
from pkg.conversion_gen import ConversionGen
from pkg.make_cmd import MakeCmd

WORKING_DIR = "workingspace"

PARSER = argparse.ArgumentParser()
PARSER.add_argument(
    "--prev-ver",
    type=str,
    required=True,
    help="Previous version. This is your existing hub which we are converting to a new spoke. If you have only one existing API version, then use that.",
)
PARSER.add_argument(
    "--new-ver",
    type=str,
    required=True,
    help="New version to create. This will be your new hub.",
)
PARSER.add_argument(
    "--most-recent-spoke",
    type=str,
    dest="most_recent_spoke",
    required=False,
    help="If you have an existing, most recent spoke that is just before the version in --prev-ver, then tell me what it is.",
)
PARSER.add_argument(
    "--repo",
    "-r",
    type=str,
    required=True,
    help="Git repository URL which has the APIs to be bumped.",
)
PARSER.add_argument(
    "--branch",
    "-b",
    type=str,
    required=False,
    help="Branch name to create. Default is 'api-<new-ver>'",
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

SUBPARSERS = PARSER.add_subparsers(help="COMMANDS", dest="cmd", required=True)

# An "all" command. This runs all of the steps, in order. Before it begins it
# figures out whether it should start with the first step or if it should pick
# up some later step.
ALL_PARSER = SUBPARSERS.add_parser("all", help="Do all steps")

# A "step" command. This attempts to figure out which step should happen
# next, and do that.
STEP_PARSER = SUBPARSERS.add_parser("step", help="Do only the next step")


def main():
    """main"""

    operation_order = [
        ["create-apis", create_apis],
        ["copy-api-content", copy_api_content],
        ["mv-webhooks", mv_webhooks],
        ["conversion-webhooks", conversion_webhooks],
        ["conversion-gen", conversion_gen],
        ["bump-controllers", bump_controllers],
        ["bump-apis", bump_apis],
        ["auto-gens", auto_gens],
    ]

    omit_step = os.getenv("OMIT_STEP")
    if omit_step is not None:
        # Just in case a step has nothing to do when it runs on your repo, you
        # have this debug aid to skip that step.
        found = False
        op_order = operation_order.copy()
        for x in op_order.copy():
            if x[0] == omit_step:
                op_order.remove(x)
                found = True
                break
        if not found:
            print(f"Did not find a step named '{omit_step}'.")
            sys.exit(1)
        operation_order = op_order

    args = PARSER.parse_args()
    if args.dryrun or args.nocommit:
        args.cmd = "step"

    gitcli = GitCLI(args.dryrun, args.nocommit)
    gitcli.clone_and_cd(args.repo, args.workdir)

    project = Project(args.dryrun)
    cgen = ConversionGen(
        args.dryrun, project, args.prev_ver, args.new_ver, args.most_recent_spoke
    )
    makecmd = MakeCmd(args.dryrun, project, args.prev_ver, args.new_ver)

    validate_args(args, cgen)

    if args.branch is None:
        args.branch = f"api-{args.new_ver}"
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

    cmd_elem = find_next_cmd(gitcli, operation_order)
    if cmd_elem is None:
        print("Unable to determine the next command.")
        sys.exit(1)
    if len(cmd_elem) == 0:
        print("The last command has been done.")
        sys.exit(1)
    if args.this_branch and cmd_elem == operation_order[0]:
        if os.getenv("USE_EXISTING_WORKAREA") is None:
            print("Arg --this_branch is allowed only after the first step.")
            sys.exit(1)

    while len(cmd_elem) > 0:
        prologue(gitcli, cmd_elem, operation_order)
        done = cmd_elem[1](cgen, makecmd, gitcli, cmd_elem[0], project, args)
        if done is False:
            print(f"\nStop on incomplete step {cmd_elem[0]}\n")
            break
        if args.cmd != "all":
            break
        # The ./PROJECT file will be modified in some steps, either directly by this
        # tool or when it runs the kubebuilder command, so reload it for each step.
        project = Project(args.dryrun)
        cmd_elem = find_next_cmd(gitcli, operation_order)


def create_apis(cgen, makecmd, git, stage, project, args):
    """
    Create a new hub API for each Kind.
    """

    _ = makecmd
    createapis = CreateApis(
        args.dryrun,
        project,
        args.prev_ver,
        args.new_ver,
        cgen.preferred_api_alias(),
        cgen.module(),
    )
    if createapis.prev_is_hub() is False:
        print("Arg --prev_ver must point to the current hub.")
        return False
    createapis.create()
    cgen.fix_kubebuilder_import_alias()
    createapis.commit_create_api(git, stage)
    return True


def copy_api_content(cgen, makecmd, git, stage, project, args):
    """
    Copy the previous hub API content to the new hub API.
    """

    createapis = CreateApis(
        args.dryrun,
        project,
        args.prev_ver,
        args.new_ver,
        cgen.preferred_api_alias(),
        cgen.module(),
    )
    createapis.copy_content(git)
    createapis.edit_new_api_files()
    createapis.remove_previous_storage_version()
    createapis.set_storage_version()
    createapis.add_conversion_schemebuilder()

    makecmd.fmt()
    createapis.commit_copy_api_content(git, stage)
    return True


def mv_webhooks(cgen, makecmd, git, stage, project, args):
    """
    Move the webhooks from the previous hub to the new hub.
    """

    webhooks = MvWebhooks(
        args.dryrun,
        project,
        args.prev_ver,
        args.new_ver,
        cgen.preferred_api_alias(),
        cgen.module(),
    )
    webhooks.edit_go_files()
    webhooks.edit_manifests()
    webhooks.mv_project_webhooks()

    makecmd.fmt()
    webhooks.commit(git, stage)
    return True


def conversion_webhooks(cgen, makecmd, git, stage, project, args):
    """
    Add a conversion webhook to anything in the new hub that needs it.
    """

    webhooks = ConversionWebhooks(
        args.dryrun,
        project,
        args.prev_ver,
        args.new_ver,
        cgen.preferred_api_alias(),
        cgen.module(),
    )
    webhooks.create()
    cgen.fix_kubebuilder_import_alias()
    webhooks.hub()
    webhooks.enable_in_crd()
    webhooks.add_fuzz_tests()

    makecmd.fmt()
    webhooks.commit(git, stage)
    return True


def conversion_gen(cgen, makecmd, git, stage, project, args):
    """
    Create the spoke conversion routines and tests.
    """
    _ = project
    _ = args

    cgen.mk_doc_go()
    cgen.mk_spoke()
    cgen.mk_spoke_fuzz_test()
    cgen.mk_conversion_webhook_suite_test()

    makecmd.fmt()
    cgen.commit(git, stage)
    return True


def bump_controllers(cgen, makecmd, git, stage, project, args):
    """
    Bump controllers to new hub version
    """

    controllers = Controllers(
        args.dryrun,
        project,
        args.prev_ver,
        args.new_ver,
        cgen.preferred_api_alias(),
        cgen.module(),
    )
    controllers.run()
    controllers.edit_util_conversion_test()

    makecmd.fmt()
    controllers.commit_bump_controllers(git, stage)
    return True


def bump_apis(cgen, makecmd, git, stage, project, args):
    """
    Bump earlier spoke APIs to new hub version.
    """

    controllers = Controllers(
        args.dryrun,
        project,
        args.prev_ver,
        args.new_ver,
        cgen.preferred_api_alias(),
        cgen.module(),
        "api",
    )
    bumped = controllers.bump_earlier_spokes()

    makecmd.fmt()
    if bumped:
        controllers.commit_bump_apis(git, stage)
    else:
        git.verify_clean()
    return bumped


def auto_gens(cgen, makecmd, git, stage, project, args):
    """
    Make auto-generated files.
    """
    _ = cgen
    _ = project
    _ = args

    makecmd.update_spoke_list()
    makecmd.manifests()
    makecmd.generate()
    makecmd.generate_go_conversions()
    makecmd.fmt()

    makecmd.commit(git, stage)
    return True


def find_next_cmd(git, operation_order):
    """
    Determine which step is the next one to execute.
    """

    prev_cmd_str = git.get_previous()
    if prev_cmd_str is None:
        return operation_order[0]
    next_cmd_elem = operation_order[-1]
    if prev_cmd_str == next_cmd_elem[0]:
        return []
    rev_list = operation_order[::-1]
    for x in rev_list:
        if x[0] == prev_cmd_str:
            return next_cmd_elem
        next_cmd_elem = x
    return None


def prologue(git, cmd_elem, operation_order):
    """
    Verify that steps are being done in the proper order.
    """

    print(f"\nExecuting {cmd_elem[0]}\n\n")
    git.verify_clean()
    op_cmd_list = [c[0] for c in operation_order]
    git.expect_previous(cmd_elem[0], op_cmd_list)


def validate_args(args, cgen):
    """Validate the commandline args"""

    if args.most_recent_spoke:
        if not cgen.is_spoke(args.most_recent_spoke):
            print(f"API --most-recent-spoke {args.most_recent_spoke} is not a spoke.")
            sys.exit(1)
        if args.most_recent_spoke in [args.prev_ver, args.new_ver]:
            print(
                "API --most-recent-spoke must not be the same as --prev_ver or --new_ver"
            )
            sys.exit(1)

    if args.prev_ver == args.new_ver:
        print("API --prev-ver and --new-ver must not be the same")
        sys.exit(1)


if __name__ == "__main__":
    main()

sys.exit(0)
