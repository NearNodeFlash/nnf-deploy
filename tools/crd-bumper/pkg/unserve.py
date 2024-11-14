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


class Unserve:
    """Tools to mark an API as unserved."""

    def __init__(self, dryrun, project, spoke_ver, preferred_alias):
        self._dryrun = dryrun
        self._project = project
        self._spoke_ver = spoke_ver
        self._preferred_alias = self.set_preferred_alias(preferred_alias)

    def set_preferred_alias(self, preferred_alias):
        """
        Take the preferred alias if it's provided, else grab it from the
        first Kind of this API.
        """
        if preferred_alias is None:
            # Pick the first kind, use its group.
            kinds = self._project.kinds(self._spoke_ver)
            group = self._project.group(kinds[0], self._spoke_ver)
        else:
            group = preferred_alias
        return group

    def set_unserved(self):
        """
        Set the kubebuilder:unservedversion marker in the specified spoke API,
        for any Kind that does not yet have it.
        """

        kinds = self._project.kinds(self._spoke_ver)
        for kind in kinds:
            fname = f"api/{self._spoke_ver}/{kind.lower()}_types.go"
            if os.path.isfile(fname):
                fu = FileUtil(self._dryrun, fname)
                found = fu.find_in_file("+kubebuilder:unservedversion")
                if found:
                    continue
                # Prefer to pair with kubebuilder:subresource:status, but fall back
                # to kubebuilder:object:root=true if status cannot be found.
                line = fu.find_in_file("+kubebuilder:subresource:status")
                if line is None:
                    line = fu.find_in_file("+kubebuilder:object:root=true")
                if line is None:
                    raise LookupError(
                        f"Unable to place kubebuilder:unservedversion in {fname}"
                    )
                fu.replace_in_file(line, f"""{line}\n// +kubebuilder:unservedversion""")
                fu.store()

    def modify_conversion_webhook_suite_test(self):
        """
        Modify the suite test that exercises the conversion webhook.

        Update the tests for the specified spoke API.

        Recall that this verifies that the conversion routines are accessed via
        the conversion webhook, and that is not intended to be an exhaustive
        conversion test.

        WARNING WARNING: This contains a series of multi-line Python f-strings
        which contain large chunks of Go code. So it's confusing. Stay sharp.
        """

        conv_go = "internal/controller/conversion_test.go"
        if not os.path.isfile(conv_go):
            print(f"NOTE: Unable to find {conv_go}!")
            return

        fu = FileUtil(self._dryrun, conv_go)

        # An ACTION note to be added to each test that we think should be removed.
        action_note = f"// ACTION: {self._spoke_ver} is no longer served, and this test can be removed."
        # Pattern to find the "It()" method so we can change it to "PIt()".
        pat = r"^(\s+)It(\(.*)"
        kinds = self._project.kinds(self._spoke_ver)
        for kind in kinds:
            spec = fu.find_in_file(
                f"""It("reads {kind} resource via hub and via spoke {self._spoke_ver}", func()"""
            )
            if spec is not None:
                newspec = spec
                m = re.search(pat, spec)
                if m is not None:
                    newspec = f"{m.group(1)}PIt{m.group(2)}"

                # Wake up!  Multi-line f-string:
                template = f"""
		It("is unable to read {kind} resource via spoke {self._spoke_ver}", func() {{
			resSpoke := &{self._preferred_alias}{self._spoke_ver}.{kind}{{}}
            Expect(k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(resHub), resSpoke)).ToNot(Succeed())
        }})
"""  # END multi-line f-string.

                fu.replace_in_file(spec, f"{template}\n{newspec}\n{action_note}\n")
        fu.store()

    def commit(self, git, stage):
        """Create a commit message."""

        msg = f"""Mark the {self._spoke_ver} API as unserved.

ACTION: Address the ACTION comments in internal/controller/conversion_test.go.

ACTION: Begin by running "make vet". Repair any issues that it finds.
  Then run "make test" and continue repairing issues until the tests
  pass.
"""
        git.commit_stage(stage, msg)
