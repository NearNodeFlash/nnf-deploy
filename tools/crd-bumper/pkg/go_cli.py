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

import shlex
import subprocess


class GoCLI:
    """Go Commands"""

    def __init__(self, dryrun):
        self._dryrun = dryrun

    def get(self, module, ver):
        """Execute a go-get."""

        cmd = f"go get {module}@{ver}"
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
                raise RuntimeError(f"Failure in command: {res.stderr}")

    def tidy(self):
        """Execute a go-mod-tidy."""

        cmd = "go mod tidy"
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
                raise RuntimeError(f"Failure in command: {res.stderr}")

    def vendor(self):
        """Execute a go-mod-vendor."""

        cmd = "go mod vendor"
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
                raise RuntimeError(f"Failure in command: {res.stderr}")
