#!/usr/bin/env python3

# Copyright 2025 Hewlett Packard Enterprise Development LP
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

"""Mark the specified CRD version as served, after it had been marked as unserved."""

import argparse
import sys

from pkg.conversion_gen import ConversionGen
from pkg.git_cli import GitCLI
from pkg.make_cmd import MakeCmd
from pkg.project import Project
from pkg.unserve import ReServe
from pkg.hub_spoke_util import HubSpokeUtil

WORKING_DIR = "workingspace"
BRANCH_SUFFIX = "re-serve"

PARSER = argparse.ArgumentParser()
PARSER.add_argument(
    "--spoke-ver",
    type=str,
    required=True,
    help="Spoke API version to mark as served.",
)
PARSER.add_argument(
    "--repo",
    "-r",
    type=str,
    required=True,
    help="Git repository URL which has the Go code that provides the APIs.",
)
PARSER.add_argument(
    "--start-branch",
    type=str,
    help="Branch or tag to checkout first, if not master.",
)
PARSER.add_argument(
    "--branch",
    "-b",
    type=str,
    required=False,
    help=f"Branch name to create. Default is 'api-<spoke_ver>-{BRANCH_SUFFIX}'",
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


def main():
    """main"""

    args = PARSER.parse_args()

    gitcli = GitCLI(args.dryrun, args.nocommit)
    gitcli.clone_and_cd(args.repo, args.workdir)
    if args.start_branch is not None:
        gitcli.checkout_branch(args.start_branch, False)

    project = Project(args.dryrun)

    cgen = ConversionGen(args.dryrun, project, args.spoke_ver, None, None)
    makecmd = MakeCmd(args.dryrun, None, None, None)

    if args.branch is None:
        args.branch = f"api-{args.spoke_ver}-{BRANCH_SUFFIX}"
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

    if not HubSpokeUtil.is_spoke(args.spoke_ver):
        print(f"API --spoke-ver {args.spoke_ver} is not a spoke.")
        sys.exit(1)

    re_serve_api(args, project, makecmd, gitcli, cgen.preferred_api_alias())


def re_serve_api(args, project, makecmd, git, preferred_api_alias):
    """Mark the specified API version as served."""

    re_serve = ReServe(args.dryrun, project, args.spoke_ver, preferred_api_alias)

    print(f"Updating files to mark API {args.spoke_ver} as served.")

    re_serve.set_served()
    re_serve.modify_conversion_webhook_suite_test()

    makecmd.manifests()
    makecmd.generate()
    makecmd.generate_go_conversions()
    makecmd.fmt()
    makecmd.clean_bin()

    re_serve.commit(git, "re-serve-api")


if __name__ == "__main__":
    main()

sys.exit(0)
