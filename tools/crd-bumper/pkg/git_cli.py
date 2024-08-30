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

import os
import shlex
import subprocess

from .copyright import Copyright


class GitCLI:
    """Git Commands"""

    def __init__(self, dryrun, nocommit):
        self._dryrun = dryrun
        if dryrun:
            nocommit = True
        self._nocommit = nocommit

    def checkout_branch(self, branch):
        """Checkout a named branch."""

        if branch in ["main", "master", "releases/v0"]:
            raise ValueError("Branch name must not be main, master, or releases/v0")
        self.verify_clean()
        cmd = f"git checkout -b {branch}"
        if self._dryrun:
            print(f"Dryrun: {cmd}")
        else:
            print(f"Run: {cmd}")
            res = subprocess.run(
                shlex.split(cmd),
                capture_output=True,
                text=True,
                check=False,
            )
            if res.returncode != 0:
                raise RuntimeError(f"Unable to checkout branch {branch}: {res.stderr}")

    def mv(self, src, dst):
        """Execute a git-mv."""

        cmd = f"git mv {src} {dst}"
        if self._dryrun:
            print(f"Dryrun: {cmd}")
        else:
            print(f"Run: {cmd}")
            res = subprocess.run(
                shlex.split(cmd),
                capture_output=True,
                text=True,
                check=False,
            )
            if res.returncode != 0:
                raise RuntimeError(
                    f"Unable to execute command to move code: {res.stderr}"
                )

    def _add_files(self):
        """Execute a git-add."""

        self._update_copyrights()

        cmd = "git add -A"
        if self._nocommit:
            print(f"Dryrun: {cmd}")
        else:
            print(f"Run: {cmd}")
            res = subprocess.run(
                shlex.split(cmd),
                capture_output=True,
                text=True,
                check=False,
            )
            if res.returncode != 0:
                raise RuntimeError(f"Unable to add files: {res.stderr}")

    def _update_copyrights(self):
        """Find all modified files, attempt to update the copyright in each one."""

        cmd = "git status -s"
        if self._dryrun:
            print(f"Dryrun: {cmd}")
        else:
            print(f"Run: {cmd}")
            res = subprocess.run(
                shlex.split(cmd),
                capture_output=True,
                text=True,
                check=False,
            )
            if res.returncode != 0:
                raise RuntimeError(f"Unable to query git status: {res.stderr}")
            cright = Copyright(self._dryrun)
            for x in res.stdout.split("\n"):
                if len(x) == 0:
                    break
                fname = x.split()[-1]
                if not fname.startswith("github/"):
                    if os.path.isfile(fname):
                        cright.update(fname)

    def verify_clean(self):
        """Determine whether there are any uncommitted changes or untracked files."""

        cmd = "git status -s"
        if self._dryrun:
            print(f"Dryrun: {cmd}")
        else:
            print(f"Run: {cmd}")
            res = subprocess.run(
                shlex.split(cmd),
                capture_output=True,
                text=True,
                check=False,
            )
            if res.returncode != 0:
                raise RuntimeError(
                    f"Unable to verify that workarea is clean: {res.stderr}"
                )
            status = res.stdout.strip()
            if len(status) > 0:
                raise RuntimeError(f"Workarea is not clean: {res.stdout}")

    def commit_stage(self, operation, msg_in):
        """
        Commit the current changes, adding a marker to the commit message to
        identify this step.

        This marker can be queried later in self.expect_previous().
        """

        self._add_files()
        msg = f"""CRDBUMPER-{operation}\n\n{msg_in}"""
        cmd = f"git commit -s -m '{msg}'"
        if self._nocommit:
            print(f"Dryrun: {cmd}")
        else:
            print(f"Run: {cmd}")
            res = subprocess.run(
                shlex.split(cmd),
                capture_output=True,
                text=True,
                check=False,
            )
            if res.returncode != 0:
                raise RuntimeError(f"Unable to commit: {res.stdout}\n{res.stderr}")
            self.verify_clean()

    def get_previous(self):
        """
        Get the previous commit operation, if it was one of ours.
        """

        cmd = "git log -1 --format='%B'"
        res = subprocess.run(
            shlex.split(cmd),
            capture_output=True,
            text=True,
            check=False,
        )
        if res.returncode != 0:
            raise RuntimeError(f"Unable to get log: {res.stderr}")
        operation = res.stdout.strip().split("\n", maxsplit=1)[0]
        if operation.startswith("CRDBUMPER-"):
            # Return the stage symbol that follows the CRDBUMPER token.
            return operation.split("-", 1)[1]
        return None

    def expect_previous(self, next_operation, operation_order):
        """
        Verify that the previous commit is the one we expect to follow the next
        desired operation.

        This allows us to enforce the order of the operations.
        """

        prev_operation = None
        for x in operation_order:
            if x == next_operation:
                break
            prev_operation = x

        if prev_operation is not None:
            prev_cmd = self.get_previous()
            prev_cmd_token = f"CRDBUMPER-{prev_cmd}"
            expect_token = f"CRDBUMPER-{prev_operation}"
            if prev_cmd_token != expect_token:
                raise ValueError(
                    f"Operation {next_operation} wants to build on {expect_token}, but found that the previous operation was {prev_cmd_token}."
                )

    def clone_and_cd(self, repo, workdir):
        """Clone the specified repo and 'cd' into it."""

        if os.path.isdir(workdir) is False:
            os.mkdir(workdir)
        os.chdir(workdir)

        newdir = os.path.basename(repo).removesuffix(".git")
        if os.path.isdir(newdir) is False:
            cmd = f"git clone {repo}"
            if self._dryrun:
                print(f"Dryrun: {cmd}")
                print(f"Dryrun: cd {newdir}")
            else:
                print(f"Run: {cmd}")
                res = subprocess.run(
                    shlex.split(cmd),
                    capture_output=True,
                    text=True,
                    check=False,
                )
                if res.returncode != 0:
                    raise RuntimeError(f"Unable to clone repo {repo}: {res.stderr}")

                if os.path.isdir(newdir) is False:
                    raise FileNotFoundError(
                        f"Expected to find directory ({newdir}) after clone."
                    )
        os.chdir(newdir)
