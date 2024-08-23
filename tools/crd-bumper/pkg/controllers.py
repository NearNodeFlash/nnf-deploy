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

from .project import Project
from .fileutil import FileUtil


class Controllers:

    def __init__(
        self,
        dryrun,
        project,
        prev_ver,
        new_ver,
        preferred_alias=None,
        module=None,
        topdir=None,
    ):
        if not isinstance(project, Project):
            raise Exception("need a Project")
        self._dryrun = dryrun
        self._project = project
        self._prev_ver = prev_ver
        self._new_ver = new_ver
        self._preferred_alias = preferred_alias
        self._module = module
        self._topdir = topdir

    def has_earlier_spokes(self):

        for _, dir_names, _ in os.walk("api", followlinks=False):
            if len(dir_names) > 2:
                return True
        return False

    def bump_earlier_spokes(self):

        earlier_spokes = self.has_earlier_spokes()
        if earlier_spokes:
            self.run()
        return earlier_spokes

    def edit_util_conversion_test(self):
        """
        Update the util/conversion fuzz test to point at the new hub.
        """

        kinds = self._project.kinds(self._prev_ver)
        if len(kinds) == 0:
            raise Exception(f"Nothing found at version {self._prev_ver}")
        self._walk_files("github/cluster-api/util/conversion", kinds)

    def run(self):

        kinds = self._project.kinds(self._prev_ver)
        if len(kinds) == 0:
            raise Exception(f"Nothing found at version {self._prev_ver}")

        if self._topdir is not None:
            self._walk_files(self._topdir, kinds)
        else:
            for top in ["internal/controller", "controllers"]:
                if os.path.isdir(top):
                    self._walk_files(top, kinds)

    def _walk_files(self, top, kinds):
        for root, _, f_names in os.walk(top, followlinks=False):
            for fname in f_names:
                this_api = os.path.basename(root)
                full_path = os.path.join(root, fname)

                if fname.startswith("zz_"):
                    # Skip generated files. Appropriate makefile targets will be used
                    # to regenerate them.
                    continue
                if top == "api" and this_api == self._new_ver:
                    # Don't try to fix the new hub; it's done already.
                    continue

                if top == "api" and fname == "doc.go":
                    self._conversiongen_marker(full_path, this_api)
                elif top == "api" and fname == "conversion.go":
                    self._update_spoke_conversion(kinds, full_path, this_api)
                elif top == "internal/controller" and fname == "suite_test.go":
                    self._update_suite_test(kinds, full_path)
                elif top == "internal/controller" and fname == "conversion_test.go":
                    pass
                else:
                    self._point_at_new_hub(kinds, full_path)

    def _point_at_new_hub(self, kinds, path):

        fu = FileUtil(self._dryrun, path)
        for k in kinds:
            if self._preferred_alias is None:
                group = self._project.group(k, self._prev_ver)
            else:
                group = self._preferred_alias

            # Find the import.
            line = fu.find_in_file(
                f'{self._prev_ver} "{self._module}/api/{self._prev_ver}"'
            )
            if line is not None:
                # Rewrite the import statement.
                # Before: '\tdwsv1alpha1 "github.com/hewpack/dws/api/v1alpha1"'
                # After:  '\tdwsv1alpha2 "github.com/hewpack/dws/api/v1alpha2"'
                line2 = line.replace(self._prev_ver, self._new_ver)
                fu.replace_in_file(line, line2)

            # This matches: dwsv1alpha1. (yes, dot)
            fu.replace_in_file(f"{group}{self._prev_ver}.", f"{group}{self._new_ver}.")
            fu.store()

    def _update_suite_test(self, kinds, path):

        fu = FileUtil(self._dryrun, path)

        if self._preferred_alias is None:
            # Pick the first kind, use its group.
            group = self._project.group(kinds[0], self._prev_ver)
        else:
            group = self._preferred_alias

        # Find the import.
        line = fu.find_in_file(
            f'{self._prev_ver} "{self._module}/api/{self._prev_ver}"'
        )
        if line is not None:
            # Add a new import statement, using the previous for the pattern.
            # Prev: '\tdwsv1alpha1 "github.com/hewpack/dws/api/v1alpha1"'
            # New:  '\tdwsv1alpha2 "github.com/hewpack/dws/api/v1alpha2"'
            line2 = line.replace(self._prev_ver, self._new_ver)
            fu.replace_in_file(line, f"{line}\n{line2}")

        # Add the new scheme.
        scaffold_scheme = fu.find_in_file("+kubebuilder:scaffold:scheme")
        new = f"\terr = {group}{self._new_ver}.AddToScheme(scheme.Scheme)\n\tExpect(err).NotTo(HaveOccurred())"
        fu.replace_in_file(scaffold_scheme, f"{new}\n\n{scaffold_scheme}")

        # Switch the webhook manager setup from the old hub to the new hub.
        for k in kinds:
            if self._preferred_alias is None:
                group = self._project.group(k, self._prev_ver)
            else:
                group = self._preferred_alias
            fu.replace_in_file(
                f"&{group}{self._prev_ver}.{k}", f"&{group}{self._new_ver}.{k}"
            )
        fu.store()

    def _conversiongen_marker(self, path, this_api):
        """
        Update the k8s:conversion-gen marker to point at the new hub.
        """

        if this_api == self._prev_ver:
            # Adjust only the established spokes; the new spoke is already correct.
            return

        fu = FileUtil(self._dryrun, path)
        line = fu.find_in_file("k8s:conversion-gen")
        if line is not None:
            line2 = f"// +k8s:conversion-gen={self._module}/api/{self._new_ver}"
            fu.replace_in_file(line, line2)
            fu.store()

    def _update_spoke_conversion(self, kinds, path, this_api):

        if this_api == self._prev_ver:
            # Adjust only the established spokes; the new spoke is already correct.
            return

        if self._preferred_alias is None:
            # Pick the first kind, use its group.
            group = self._project.group(kinds[0], self._prev_ver)
        else:
            group = self._preferred_alias

        fu = FileUtil(self._dryrun, path)

        # This has tab chars to match the "import".
        line = fu.find_in_file(f"\t{group}{self._prev_ver} ")
        if line is not None:
            # Rewrite the import statement.
            # Before: '\tdwsv1alpha1 "github.com/hewpack/dws/api/v1alpha1"'
            # After:  '\tdwsv1alpha2 "github.com/hewpack/dws/api/v1alpha2"'
            pat = rf'^(\s+){group}{self._prev_ver}(\s+".*/){self._prev_ver}"'
            m = re.search(pat, line)
            if m is not None:
                new = f'{m.group(1)}{group}{self._new_ver}{m.group(2)}{self._new_ver}"'
                fu.replace_in_file(line, new)

        # This matches: dwsv1alpha1. (yes, dot)
        fu.replace_in_file(f"{group}{self._prev_ver}.", f"{group}{self._new_ver}.")

        # The Convert_*() functions:
        fu.replace_in_file(f"_{self._prev_ver}_", f"_{self._new_ver}_")
        fu.store()

    def commit_bump_controllers(self, git, stage):
        """
        Create commit with a message about pointing the controllers at the new hub.
        """

        msg = f"""Point controllers at new hub {self._new_ver}

Point conversion fuzz test at new hub. These routines are still
valid for the new hub because it is currently identical to the
previous hub."""

        non_local = self._project.controllers_with_nonlocal_api()
        if len(non_local) > 0:
            msg = (
                msg
                + f"""

ACTION: Some controllers may have been referencing one of these
  non-local APIs. Verify that these APIs are being referenced
  by their correct versions:
  {", ".join(non_local)}

"""
            )

        git.commit_stage(stage, msg)

    def commit_bump_apis(self, git, stage):
        """
        Create commit with a message about pointing the spokes at the new hub.
        """

        msg = f"""Point earlier spoke APIs at new hub {self._new_ver}.

The conversion_test.go and the ConvertTo()/ConvertFrom() routines in
conversion.go are still valid for the new hub because it is currently
identical to the previous hub.

Update the k8s:conversion-gen marker in doc.go to point to the new hub."""

        non_local = self._project.controllers_with_nonlocal_api()
        if len(non_local) > 0:
            msg = (
                msg
                + f"""

ACTION: Some API libraries may have been referencing one of these
  non-local APIs. Verify that these APIs are being referenced
  by their correct versions:
  {", ".join(non_local)}

"""
            )

        git.commit_stage(stage, msg)