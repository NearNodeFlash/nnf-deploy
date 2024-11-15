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
import subprocess
import shlex
import shutil

from .project import Project
from .fileutil import FileUtil
from .hub_spoke_util import HubSpokeUtil


class CreateApis:
    """Create the new API versions."""

    def __init__(self, dryrun, project, prev_ver, new_ver, preferred_alias, module):
        if not isinstance(project, Project):
            raise TypeError("need a Project")
        self._dryrun = dryrun
        self._project = project
        self._prev_ver = prev_ver
        self._new_ver = new_ver
        self._preferred_alias = preferred_alias
        self._module = module

    def prev_is_hub(self):
        """
        If we begin with more than one API version, then we must begin with the
        one that is the hub.
        """

        for _, dir_names, _ in os.walk("api", followlinks=False):
            if len(dir_names) > 1:
                return HubSpokeUtil.is_hub(self._prev_ver)
        return True

    def create(self):
        """For each kind, create a new API version using kubebuilder."""

        kinds = self._project.kinds(self._prev_ver)
        if len(kinds) == 0:
            raise ValueError(f"Nothing found at version {self._prev_ver}")
        kinds_new_ver = self._project.kinds(self._new_ver)

        for k in kinds:
            if k not in kinds_new_ver:
                group = self._project.group(k, self._prev_ver)
                cmd = f"kubebuilder create api --group {group} --version {self._new_ver} --kind {k} --resource --controller=false"

                if self._dryrun:
                    print(f"Dryrun: {cmd}")
                else:
                    print(f"Running: {cmd}")
                    res = subprocess.run(
                        shlex.split(cmd),
                        capture_output=True,
                        text=True,
                        check=False,
                    )
                    if res.returncode != 0:
                        raise RuntimeError(
                            f"Unable to create API for {k}.{self._new_ver}:\n{res.stderr}"
                        )

    def copy_content(self, git):
        """Copy the API content from the previous hub to the new hub."""

        kinds_prev_ver = self._project.kinds(self._prev_ver)
        if len(kinds_prev_ver) == 0:
            raise ValueError(f"Nothing found at version {self._prev_ver}")
        kinds_new_ver = self._project.kinds(self._new_ver)

        copied = {}
        for k in kinds_new_ver:
            if k in kinds_prev_ver:
                src = f"api/{self._prev_ver}/{k.lower()}_types.go"
                dst = f"api/{self._new_ver}/{k.lower()}_types.go"
                if os.path.isfile(src) and os.path.isfile(dst):
                    shutil.copy2(src, dst, follow_symlinks=False)
                    copied[src] = True

        # Now copy any extra libs that may have been written to support those.
        skip = [
            "doc.go",
            "groupversion_info.go",
            "conversion.go",
            "conversion_test.go",
            "zz_generated.deepcopy.go",
            "zz_generated.conversion.go",
        ]
        for root, _, f_names in os.walk(f"api/{self._prev_ver}", followlinks=False):
            for fname in f_names:
                if fname in skip or fname in copied:
                    continue
                full_path = os.path.join(root, fname)
                dst_path = full_path.replace(
                    f"api/{self._prev_ver}/", f"api/{self._new_ver}/"
                )
                if "webhook" in fname:
                    git.mv(full_path, dst_path)
                else:
                    shutil.copy2(full_path, dst_path, follow_symlinks=False)

    def remove_previous_storage_version(self):
        """
        Remove the kubebuilder:storageversion marker from the earlier spoke. It
        is now in the new hub.
        """

        kinds = self._project.kinds(self._prev_ver)
        for kind in kinds:
            fname = f"api/{self._prev_ver}/{kind.lower()}_types.go"
            if os.path.isfile(fname):
                fu = FileUtil(self._dryrun, fname)
                # It could be with or without a space, depending on the kubebuilder
                # version that wrote it.
                changed = fu.delete_from_file("//+kubebuilder:storageversion")
                if not changed:
                    changed = fu.delete_from_file("// +kubebuilder:storageversion")
                fu.store()

    def set_storage_version(self):
        """
        Set the kubebuilder:storageversion marker in the new hub, for any Kind that
        does not yet have it.
        """

        kinds = self._project.kinds(self._prev_ver)
        for kind in kinds:
            fname = f"api/{self._new_ver}/{kind.lower()}_types.go"
            if os.path.isfile(fname):
                fu = FileUtil(self._dryrun, fname)
                found = fu.find_in_file("+kubebuilder:storageversion")
                if found:
                    continue
                # Prefer to pair with kubebuilder:subresource:status, but fall back
                # to kubebuilder:object:root=true if status cannot be found.
                line = fu.find_in_file("+kubebuilder:subresource:status")
                if line is None:
                    line = fu.find_in_file("+kubebuilder:object:root=true")
                if line is None:
                    raise LookupError(
                        f"Unable to place kubebuilder:storageversion in {fname}"
                    )
                fu.replace_in_file(line, f"""{line}\n// +kubebuilder:storageversion""")
                fu.store()

    def add_conversion_schemebuilder(self):
        """
        Add the "localSchemeBuilder" variable that will be used by the generated
        conversion routines.
        """

        gv_info = f"api/{self._prev_ver}/groupversion_info.go"
        fu = FileUtil(self._dryrun, gv_info)
        line = fu.find_with_pattern("AddToScheme = SchemeBuilder.AddToScheme")
        local_var = f"""{line}

	// Used by zz_generated.conversion.go.
	localSchemeBuilder = SchemeBuilder.SchemeBuilder"""
        fu.replace_in_file(line, local_var)
        fu.store()

    def edit_new_api_files(self):
        """
        Update the API version reference in the Go files that have content that
        was relocated from the previous hub to the new hub.
        """

        kinds = self._project.kinds(self._new_ver)
        for root, _, f_names in os.walk(f"api/{self._new_ver}", followlinks=False):
            for fname in f_names:
                full_path = os.path.join(root, fname)
                if os.path.isfile(full_path):
                    fu = FileUtil(self._dryrun, full_path)
                    fu.replace_in_file(
                        f"package {self._prev_ver}", f"package {self._new_ver}"
                    )
                    for k in kinds:
                        if self._preferred_alias is None:
                            group = self._project.group(k, self._new_ver)
                        else:
                            group = self._preferred_alias

                        # This has tab chars to match the "import".
                        line = fu.find_in_file(f"\t{group}{self._prev_ver} ")
                        if line is not None:
                            # Rewrite the import statement.
                            # Before: '\tdwsv1alpha1 "github.com/hewpack/dws/api/v1alpha1"'
                            # After:  '\tdwsv1alpha2 "github.com/hewpack/dws/api/v1alpha2"'
                            line2 = f'\t{group}{self._new_ver} "{self._module}/api/{self._new_ver}"'
                            fu.replace_in_file(line, line2)

                        # This matches: dwsv1alpha1.  (yes, dot)
                        fu.replace_in_file(
                            f"{group}{self._prev_ver}.", f"{group}{self._new_ver}."
                        )
                    fu.store()

    def commit_create_api(self, git, stage):
        """
        Create commit with a message that describes the creation of the new APIs.
        """

        msg = f"""Create {self._new_ver} APIs.

This used \"kubebuilder create api --resource --controller=false\"
for each API."""

        git.commit_stage(stage, msg)

    def commit_copy_api_content(self, git, stage):
        """
        Create commit with a message that describes the copying of the API content.
        """

        msg = f"""Copy API content from {self._prev_ver} to {self._new_ver}.

Move the kubebuilder:storageversion marker from {self._prev_ver} to {self._new_ver}.

Set localSchemeBuilder var in api/{self._prev_ver}/groupversion_info.go
to satisfy zz_generated.conversion.go."""

        git.commit_stage(stage, msg)
