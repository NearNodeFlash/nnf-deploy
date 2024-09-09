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

from .project import Project
from .fileutil import FileUtil


class MakeCmd:
    """Run make commands or updating the Makefile."""

    def __init__(self, dryrun, project, prev_ver, new_ver):
        if not isinstance(project, Project):
            raise TypeError("need a Project")
        self._dryrun = dryrun
        self._project = project
        self._prev_ver = prev_ver
        self._new_ver = new_ver

    def update_spoke_list(self):
        """
        Update the list of spokes for the 'generate-go-conversions' target in
        the makefile.
        """
        spokary = []
        for root, dir_names, _ in os.walk("api", followlinks=False):
            for d in dir_names:
                if d != self._new_ver:
                    spokary.append(f"./{root}/{d}")
        spokes = " ".join(sorted(spokary))
        fu = FileUtil(self._dryrun, "Makefile")
        src_dirs = fu.find_with_pattern("^SRC_DIRS=")
        fu.replace_in_file(src_dirs, f"SRC_DIRS={spokes}")
        fu.store()

    def manifests(self):
        """Execute 'make manifests'"""

        cmd = "make manifests"
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
                raise RuntimeError(f"Unable to {cmd}: {res.stderr}")

    def generate(self):
        """Execute 'make generate'"""

        cmd = "make generate"
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
                raise RuntimeError(f"Unable to {cmd}: {res.stderr}")

    def generate_go_conversions(self):
        """Execute 'make generate-go-conversions'"""

        cmd = "make generate-go-conversions"
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
                raise RuntimeError(f"Unable to {cmd}: {res.stderr}")

    def fmt(self):
        """Execute 'make fmt'"""

        cmd = "make fmt"
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
                raise RuntimeError(f"Unable to {cmd}: {res.stderr}")

    def clean_bin(self):
        """Execute 'make clean-bin'"""

        cmd = "make clean-bin"
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
                raise RuntimeError(f"Unable to {cmd}: {res.stderr}")

    def commit(self, git, stage):
        """
        Create commit with a message about the auto-generated files.
        """

        msg = """Make the auto-generated files.

Update the SRC_DIRS spoke list in the Makefile.

make manifests & make generate & make generate-go-conversions
make fmt

ACTION: If any of the code in this repo was referencing non-local
  APIs, the references to them may have been inadvertently
  modified. Verify that any non-local APIs are being referenced
  by their correct versions.

ACTION: Begin by running "make vet". Repair any issues that it finds.
  Then run "make test" and continue repairing issues until the tests
  pass.
"""

        git.commit_stage(stage, msg)
