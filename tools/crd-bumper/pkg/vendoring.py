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
import re

from .fileutil import FileUtil


class Vendor:
    """Tools for vendoring a new API version."""

    def __init__(self, dryrun, module, hub_ver, vendor_hub_ver):
        self._dryrun = dryrun
        self._module = module
        self._hub_ver = hub_ver
        self._vendor_hub_ver = vendor_hub_ver
        self._current_ver = None
        self._preferred_alias = None

    def current_api_version(self):
        """Return the current in-use API version."""
        return self._current_ver

    def uses_module(self):
        """Determine whether the specified module is vendored here."""
        path = "go.mod"
        if not os.path.isfile(path):
            return False
        fu = FileUtil(self._dryrun, path)
        line = fu.find_with_pattern(f"^\t{self._module} v")
        if line is None:
            return False
        return True

    def set_current_api_version(self):
        """Determine the version of the API currently being vendored."""

        path = f"vendor/{self._module}/api"
        for _, dir_names, _ in os.walk(path, followlinks=False):
            if len(dir_names) == 1:
                # Only one API vendored here.
                self._current_ver = dir_names[0]
                break
            raise ValueError(
                f"Expected to find one API at {path}, but found {len(dir_names)}."
            )
        if self._current_ver is None:
            raise ValueError(f"Unable to find API at {path}.")

    def verify_one_api_version(self):
        """Verify that only one API version is being vendored."""

        self._current_ver = None
        self.set_current_api_version()

    def set_preferred_api_alias(self, main_file=None):
        """
        What is this repo using as the alias for this module's API?

        In other words, which of the following import statements is preferred in
        this module, where the group, unfortunately, is "dataworkflowservices"?

           dwsv1alpha1 "github.com/DataWorkflowServices/dws/api/v1alpha"
           dataworkflowservicesv1alpha1 "github.com/DataWorkflowServices/dws/api/v1alpha"

        We'll look at cmd/main.go to get an answer.
        """

        if main_file is not None:
            fname = main_file
        else:
            fname = "cmd/main.go"
        fu = FileUtil(self._dryrun, fname)
        # Find the import.
        line = fu.find_in_file(
            f'{self._current_ver} "{self._module}/api/{self._current_ver}"'
        )
        if line is not None:
            pat = rf'^\s+(.+){self._current_ver}\s+"{self._module}/api/{self._current_ver}"'
            m = re.search(pat, line)
            if m is not None:
                self._preferred_alias = m.group(1)
        if self._preferred_alias is None:
            raise ValueError(f"Expected to find the module's alias in {fname}.")

    def update_go_file(self, full_path):
        """Bump the given Go file to point at the new hub."""
        self._point_go_files_at_new_hub(full_path)

    def update_go_files(self, top=None):
        """Walk over Go files, bumping them to point at the new hub
        If top=None then this walks over the cmd/, internal/, and api/ directories
        that kubebuilder would have put in place.
        """

        if top is not None:
            if os.path.isdir(top) is False:
                raise NotADirectoryError(f"{top} is not a directory.")
            top = [top]
        else:
            top = ["cmd", "internal/controller", "controllers"]
            if self._hub_ver is not None:
                top.append("api/" + self._hub_ver)

        for dname in top:
            if os.path.isdir(dname):
                self._walk_go_files(dname)

    def _walk_go_files(self, dirname):
        """Walk the files in the given directory, and update them to point at the new hub."""

        if os.path.isdir(dirname) is False:
            raise NotADirectoryError(f"{dirname} is not a directory")

        for root, _, f_names in os.walk(dirname, followlinks=False):
            for fname in f_names:
                full_path = os.path.join(root, fname)
                if fname.endswith(".go"):
                    self._point_go_files_at_new_hub(full_path)

    def _point_go_files_at_new_hub(self, path):
        """Update the given file to point it at the new hub."""

        fu = FileUtil(self._dryrun, path)
        group = self._preferred_alias

        # Find the import.
        pat = f'{group}{self._current_ver} "{self._module}/api/{self._current_ver}"'
        line = fu.find_in_file(pat)
        if line is not None:
            # Rewrite the import statement.
            # Before: '\tdwsv1alpha1 "github.com/hewpack/dws/api/v1alpha1"'
            # After:  '\tdwsv1alpha2 "github.com/hewpack/dws/api/v1alpha2"'
            line2 = line.replace(self._current_ver, self._vendor_hub_ver)
            fu.replace_in_file(line, line2)
        # This matches: dwsv1alpha1. (yes, dot)
        fu.replace_in_file(
            f"{group}{self._current_ver}.", f"{group}{self._vendor_hub_ver}."
        )
        fu.store()

    def update_config_files(self, top):
        """Walk over Kustomize config files, bumping them to point at the new hub."""

        if os.path.isdir(top) is False:
            raise NotADirectoryError(f"{top} is not a directory.")

        self._walk_config_files(top)

    def _walk_config_files(self, dirname):
        """Walk the files in the given directory, and update them to point at the new hub."""

        if os.path.isdir(dirname) is False:
            raise NotADirectoryError(f"{dirname} is not a directory")

        for root, _, f_names in os.walk(dirname, followlinks=False):
            for fname in f_names:
                full_path = os.path.join(root, fname)
                if fname.endswith(".yaml"):
                    self._point_config_files_at_new_hub(full_path)

    def _point_config_files_at_new_hub(self, path):
        """Update the given file to point it at the new hub."""

        fu = FileUtil(self._dryrun, path)
        group = self._preferred_alias

        pat = f"apiVersion: {group}"
        line = fu.find_in_file(pat)
        if line is not None:
            line2 = line.replace(self._current_ver, self._vendor_hub_ver)
            fu.replace_in_file(line, line2)
        fu.store()

    def commit(self, git, stage):
        """Create a commit message."""

        msg = f"""Vendor {self._vendor_hub_ver} API from {self._module}.

ACTION: If any of the code in this repo was referencing non-local
  APIs, the references to them may have been inadvertently
  modified. Verify that any non-local APIs are being referenced
  by their correct versions.

ACTION: Begin by running "make vet". Repair any issues that it finds.
  Then run "make test" and continue repairing issues until the tests
  pass.
"""
        git.commit_stage(stage, msg)
