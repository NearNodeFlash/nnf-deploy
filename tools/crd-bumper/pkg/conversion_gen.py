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
import shutil

from .project import Project
from .fileutil import FileUtil


class ConversionGen:
    """Create conversion routines and tests."""

    def __init__(self, dryrun, project, prev_ver, new_ver, most_recent_spoke):
        if not isinstance(project, Project):
            raise TypeError("need a Project")
        self._dryrun = dryrun
        self._project = project
        self._prev_ver = prev_ver
        self._new_ver = new_ver
        self._most_recent_spoke = most_recent_spoke
        self._module = None
        self.module()
        self._preferred_alias = self.preferred_api_alias()

    def is_spoke(self, ver):
        """
        Determine whether or not the 'ver' API is a spoke.
        """
        path = f"api/{ver}/conversion.go"
        if not os.path.isfile(path):
            return False
        fu = FileUtil(self._dryrun, path)
        line = fu.find_in_file(" ConvertTo(dstRaw conversion.Hub) ")
        if line is None:
            return False
        return True

    def preferred_api_alias(self):
        """
        Is this repo using the API "group" as the alias or is it using something
        else?

        In other words, which of the following import statements is preferred in
        this module, where the group, unfortunately, is "dataworkflowservices"?

           dwsv1alpha1 "github.com/DataWorkflowServices/dws/api/v1alpha"
           dataworkflowservicesv1alpha1 "github.com/DataWorkflowServices/dws/api/v1alpha"

        We'll look at cmd/main.go to get an answer.
        """

        # Pick the group of the first kind, for comparison.
        kinds = self._project.kinds(self._prev_ver)
        group = self._project.group(kinds[0], self._prev_ver)

        fname = "cmd/main.go"
        fu = FileUtil(self._dryrun, fname)
        # Find the import for prev_ver.
        line = fu.find_in_file(
            f'{self._prev_ver} "{self._module}/api/{self._prev_ver}"'
        )
        if line is not None:
            pat = rf'^\s+(.+){self._prev_ver}\s+"{self._module}/api/{self._prev_ver}"'
            m = re.search(pat, line)
            if m is not None and m.group(1) != group:
                return m.group(1)
        return None

    def fix_kubebuilder_import_alias(self):
        """
        Change the import alias that was written by kubebuiler. This code has
        a different preference for it.
        """

        if self._preferred_alias is None:
            return

        kinds = self._project.kinds(self._prev_ver)
        # Pick the first kind, use its group.
        group = self._project.group(kinds[0], self._prev_ver)

        path = "cmd/main.go"
        fu = FileUtil(self._dryrun, path)
        fu.replace_in_file(
            f"{group}{self._new_ver}", f"{self._preferred_alias}{self._new_ver}"
        )
        fu.store()

    def module(self):
        """Return the name of this Go module."""

        if self._module is not None:
            return self._module

        go_mod = "go.mod"
        if not os.path.isfile(go_mod):
            raise FileNotFoundError(f"unable to find {go_mod}")

        # Get the module name.
        fu = FileUtil(self._dryrun, go_mod)
        module_line = fu.find_with_pattern("^module ")
        if module_line is None:
            raise ValueError(f"unable to find module name in {go_mod}")
        m = re.search(r"^module\s+(.*)", module_line)
        if m is None:
            raise ValueError(f"unable to parse module name in {go_mod}")
        self._module = m.group(1)
        return self._module

    def mk_doc_go(self):
        """
        Make a doc.go for the spoke. This is where we place the Go marker that
        tells the k8s conversion-gen tool to generate conversion routines for the
        specified hub.
        """

        docgo = f"api/{self._prev_ver}/doc.go"
        if not self._dryrun:
            shutil.copy2("hack/boilerplate.go.txt", docgo, follow_symlinks=False)

        fu = FileUtil(self._dryrun, docgo)
        fu.append(
            "\n// The following tag tells conversion-gen to generate conversion routines, and\n"
        )
        fu.append("// it tells conversion-gen the name of the hub version.\n")
        fu.append(f"// +k8s:conversion-gen={self._module}/api/{self._new_ver}\n")
        fu.append(f"package {self._prev_ver}\n")
        fu.store()

    def _add_import_for_new_hub(self, kinds, fu):
        """
        Update the file to add an import for the new hub.
        """
        if not isinstance(fu, FileUtil):
            raise TypeError("need a FileUtil")

        if self._preferred_alias is None:
            # Pick the first kind, use its group.
            group = self._project.group(kinds[0], self._prev_ver)
        else:
            group = self._preferred_alias

        # This has tab chars to match the "import".
        line = fu.find_in_file(f"\t{group}{self._prev_ver} ")
        # Add a new import statement, using the previous for the pattern.
        # Prev: '\tdwsv1alpha1 "github.com/hewpack/dws/api/v1alpha1"'
        # New:  '\tdwsv1alpha2 "github.com/hewpack/dws/api/v1alpha2"'
        pat = rf'^(\s+){group}{self._prev_ver}(\s+".*/){self._prev_ver}"'
        m = re.search(pat, line)
        if m is not None:
            new = f'{m.group(1)}{group}{self._new_ver}{m.group(2)}{self._new_ver}"'
            fu.replace_in_file(line, f"{line}\n{new}")

    def mk_spoke(self):
        """
        Make a conversion.go for the spoke.

        A spoke's conversion.go contains ConvertTo()/ConvertFrom() routines. This
        replaces the conversion.go that contained Hub() routines.

        These new conversion routines do nothing special yet, because at this very
        moment the new hub API version is a direct copy of this API version.

        WARNING WARNING: This contains a series of multi-line Python f-strings
        which contain large chunks of Go code. So it's confusing. Stay sharp.
        """

        kinds = self._project.kinds(self._prev_ver)
        if self._preferred_alias is None:
            # Pick the first kind, use its group.
            group = self._project.group(kinds[0], self._prev_ver)
        else:
            group = self._preferred_alias

        spoke_go = f"api/{self._prev_ver}/conversion.go"
        if not self._dryrun:
            shutil.copy2("hack/boilerplate.go.txt", spoke_go, follow_symlinks=False)

        fu = FileUtil(self._dryrun, spoke_go)
        # Wake up!  Multi-line f-string:
        template = f"""
package {self._prev_ver}

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	{group}{self._new_ver} "{self._module}/api/{self._new_ver}"
	utilconversion "{self._module}/github/cluster-api/util/conversion"
)

var convertlog = logf.Log.V(2).WithName("convert-{self._prev_ver}")
"""  # END multi-line f-string.

        fu.append(template)

        for kind in kinds:
            if self._preferred_alias is None:
                group = self._project.group(kind, self._prev_ver)
            else:
                group = self._preferred_alias

            # Wake up!  Multi-line f-string:
            template = f"""
func (src *{kind}) ConvertTo(dstRaw conversion.Hub) error {{
	convertlog.Info("Convert {kind} To Hub", "name", src.GetName(), "namespace", src.GetNamespace())
	dst := dstRaw.(*{group}{self._new_ver}.{kind})

	if err := Convert_{self._prev_ver}_{kind}_To_{self._new_ver}_{kind}(src, dst, nil); err != nil {{
		return err
	}}

	// Manually restore data.
	restored := &{group}{self._new_ver}.{kind}{{}}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {{
		return err
	}}
	// EDIT THIS FUNCTION! If the annotation is holding anything that is
	// hub-specific then copy it into 'dst' from 'restored'.
	// Otherwise, you may comment out UnmarshalData() until it's needed.

	return nil
}}

func (dst *{kind}) ConvertFrom(srcRaw conversion.Hub) error {{
	src := srcRaw.(*{group}{self._new_ver}.{kind})
	convertlog.Info("Convert {kind} From Hub", "name", src.GetName(), "namespace", src.GetNamespace())

	if err := Convert_{self._new_ver}_{kind}_To_{self._prev_ver}_{kind}(src, dst, nil); err != nil {{
		return err
	}}

	// Preserve Hub data on down-conversion except for metadata.
	return utilconversion.MarshalData(src, dst)
}}
"""  # END multi-line f-string.

            fu.append(template)

        if self._preferred_alias is None:
            # Use the first group.
            group = self._project.group(kinds[0], self._prev_ver)
        else:
            group = self._preferred_alias

        # Wake up!  Multi-line f-string:
        template = f"""
// The List-based ConvertTo/ConvertFrom routines are never used by the
// conversion webhook, but the conversion-verifier tool wants to see them.
// The conversion-gen tool generated the Convert_X_to_Y routines, should they
// ever be needed.

func resource(resource string) schema.GroupResource {{
	return schema.GroupResource{{Group: "{group}", Resource: resource}}
}}
"""  # END multi-line f-string.

        fu.append(template)

        for kind in kinds:
            if self._preferred_alias is None:
                group = self._project.group(kind, self._prev_ver)
            else:
                group = self._preferred_alias
            # Wake up!  Multi-line f-string:
            template = f"""
func (src *{kind}List) ConvertTo(dstRaw conversion.Hub) error {{
	return apierrors.NewMethodNotSupported(resource("{kind}List"), "ConvertTo")
}}

func (dst *{kind}List) ConvertFrom(srcRaw conversion.Hub) error {{
	return apierrors.NewMethodNotSupported(resource("{kind}List"), "ConvertFrom")
}}
"""  # END multi-line f-string.

            fu.append(template)

        # enough
        fu.store()

    def _update_spoke_conversion(self, kinds, fu):
        if not isinstance(fu, FileUtil):
            raise TypeError("need a FileUtil")

        if self._preferred_alias is None:
            # Pick the first kind, use its group.
            group = self._project.group(kinds[0], self._prev_ver)
        else:
            group = self._preferred_alias

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

    def mk_spoke_fuzz_test(self):
        """
        Create a conversion_test.go for the spoke's API.

        This is the file that has the fuzz tester that converts spoke-hub-spoke and
        hub-spoke-hub to verify that the ConvertTo()/ConvertFrom() routines in
        conversion.go from mk_spoke() above do not lose data.

        The fuzz tester does not use the webhook. See
        mk_conversion_webhook_suite_test() below for that.

        WARNING WARNING: This contains a series of multi-line Python f-strings
        which contain large chunks of Go code. So it's confusing. Stay sharp.
        """

        # Pick the first kind, use its group.
        kinds = self._project.kinds(self._prev_ver)
        if self._preferred_alias is None:
            group = self._project.group(kinds[0], self._prev_ver)
        else:
            group = self._preferred_alias

        spoke_go = f"api/{self._prev_ver}/conversion_test.go"
        if not self._dryrun:
            shutil.copy2("hack/boilerplate.go.txt", spoke_go, follow_symlinks=False)

        fu = FileUtil(self._dryrun, spoke_go)
        # Wake up!  Multi-line f-string:
        template = f"""
package {self._prev_ver}

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"

	{group}{self._new_ver} "{self._module}/api/{self._new_ver}"
	utilconversion "{self._module}/github/cluster-api/util/conversion"
)

func TestFuzzyConversion(t *testing.T) {{
"""  # END multi-line f-string.

        fu.append(template)

        for kind in kinds:
            if self._preferred_alias is None:
                group = self._project.group(kind, self._prev_ver)
            else:
                group = self._preferred_alias

            # Wake up!  Multi-line f-string:
            template = f"""
	t.Run("for {kind}", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{{
		Hub:   &{group}{self._new_ver}.{kind}{{}},
		Spoke: &{kind}{{}},
	}}))
"""  # END multi-line f-string.
            fu.append(template)

        # Wake up!  Multi-line f-string:
        # pylint: disable=f-string-without-interpolation
        template = f"""
}}

// Just touch ginkgo, so it's here to interpret any ginkgo args from
// "make test", so that doesn't fail on this test file.
var _ = BeforeSuite(func() {{}})
"""  # END multi-line f-string.

        fu.append(template)

        # enough
        fu.store()

    def mk_conversion_webhook_suite_test(self):
        """
        Create a suite test that exercises the conversion webhook.

        This verifies that the conversion routines are accessed via the conversion
        webhook, and does nothing more than that.

        This is not intended to be an exhaustive conversion test. See
        mk_spoke_fuzz_test() above for that.

        WARNING WARNING: This contains a series of multi-line Python f-strings
        which contain large chunks of Go code. So it's confusing. Stay sharp.
        """

        # Pick the first kind, use its group.
        kinds = self._project.kinds(self._prev_ver)
        if self._preferred_alias is None:
            group = self._project.group(kinds[0], self._prev_ver)
        else:
            group = self._preferred_alias

        conv_go = "internal/controller/conversion_test.go"
        package = "controller"

        is_new = False
        if not os.path.isfile(conv_go):
            if not self._dryrun:
                shutil.copy2("hack/boilerplate.go.txt", conv_go, follow_symlinks=False)
                is_new = True

        fu = FileUtil(self._dryrun, conv_go)

        if is_new:
            # Wake up!  Multi-line f-string:
            template = f"""
package {package}

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	{group}{self._prev_ver} "{self._module}/api/{self._prev_ver}"
	{group}{self._new_ver} "{self._module}/api/{self._new_ver}"
	utilconversion "{self._module}/github/cluster-api/util/conversion"
)

var _ = Describe("Conversion Webhook Test", func() {{

	// Don't get deep into verifying the conversion.
	// We have api/<spoke_ver>/conversion_test.go that is digging deep.
	// We're just verifying that the conversion webhook is hooked up.

	// Note: if a resource is accessed by its spoke API, then it should
	// have the utilconversion.DataAnnotation annotation.  It will not
	// have that annotation when it is accessed by its hub API.

	// +crdbumper:scaffold:webhooksuitetest
}})
"""  # END multi-line f-string.

            fu.append(template)  # if is_new

        if not is_new:
            self._add_import_for_new_hub(kinds, fu)
            # Begin by bumping all existing Kinds from the old hub to the new hub.
            for kind in kinds:
                if self._preferred_alias is None:
                    group = self._project.group(kind, self._prev_ver)
                else:
                    group = self._preferred_alias

                # This matches: dwsv1alpha1. (yes, dot)
                fu.replace_in_file(
                    f"{group}{self._prev_ver}.", f"{group}{self._new_ver}."
                )

        # Now add any new Kinds.
        for kind in kinds:
            realgroup = self._project.group(kind, self._prev_ver)
            if self._preferred_alias is None:
                group = self._project.group(kind, self._prev_ver)
            else:
                group = self._preferred_alias

            found = fu.find_in_file(f"""Context("{kind}", func""")
            if found is not None:
                continue

            scaffold_test = fu.find_in_file("+crdbumper:scaffold:webhooksuitetest")

            # Wake up!  Multi-line f-string:
            template = f"""
	Context("{kind}", func() {{
		var resHub *{group}{self._new_ver}.{kind}

		BeforeEach(func() {{
			id := uuid.NewString()[0:8]
			resHub = &{group}{self._new_ver}.{kind}{{
				ObjectMeta: metav1.ObjectMeta{{
					Name: id,
					Namespace: corev1.NamespaceDefault,
				}},
				//Spec: {group}{self._new_ver}.{kind}Spec{{}},
			}}

			Expect(k8sClient.Create(context.TODO(), resHub)).To(Succeed())
		}})

		AfterEach(func() {{
			if resHub != nil {{
				Expect(k8sClient.Delete(context.TODO(), resHub)).To(Succeed())
				expected := &{group}{self._new_ver}.{kind}{{}}
				Eventually(func() error {{ // Delete can still return the cached object. Wait until the object is no longer present.
					return k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(resHub), expected)
				}}).ShouldNot(Succeed())
			}}
		}})

		// +crdbumper:scaffold:spoketest="{realgroup}.{kind}"
	}})
"""  # END multi-line f-string.
            fu.replace_in_file(scaffold_test, f"{template}\n{scaffold_test}")

        # Now add the tests for the new spoke, for each Kind.
        for kind in kinds:
            realgroup = self._project.group(kind, self._prev_ver)
            if self._preferred_alias is None:
                group = self._project.group(kind, self._prev_ver)
            else:
                group = self._preferred_alias

            scaffold_spoketest = fu.find_in_file(
                f'crdbumper:scaffold:spoketest="{realgroup}.{kind}"'
            )
            if scaffold_spoketest is None:
                # Don't know where to put the spoke test, so skip it.
                continue

            # Wake up!  Multi-line f-string:
            template = f"""
		It("reads {kind} resource via hub and via spoke {self._prev_ver}", func() {{
			// Spoke should have annotation.
			resSpoke := &{group}{self._prev_ver}.{kind}{{}}
			Eventually(func(g Gomega) {{
				g.Expect(k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(resHub), resSpoke)).To(Succeed())
				anno := resSpoke.GetAnnotations()
				g.Expect(anno).To(HaveLen(1))
				g.Expect(anno).Should(HaveKey(utilconversion.DataAnnotation))
			}}).Should(Succeed())

			// Hub should not have annotation.
			Eventually(func(g Gomega) {{
				g.Expect(k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(resHub), resHub)).To(Succeed())
				anno := resHub.GetAnnotations()
				g.Expect(anno).To(HaveLen(0))
			}}).Should(Succeed())
		}})
"""  # END multi-line f-string.
            fu.replace_in_file(scaffold_spoketest, f"{template}\n{scaffold_spoketest}")

        fu.store()

    def commit(self, git, stage):
        """
        Create commit with a message that describes the conversion routines.
        """

        msg = f"""Create conversion routines and tests for {self._prev_ver}.

Switch api/{self._prev_ver}/conversion.go content from hub to spoke.

These conversion.go ConvertTo()/ConvertFrom() routines are complete
and do not require manual adjustment at this time, because {self._prev_ver} is
currently identical to the new hub {self._new_ver}.

ACTION: The api/{self._prev_ver}/conversion_test.go may need to be
  manually adjusted for your needs, especially if it has been manually
  adjusted in earlier spokes.

ACTION: Any new tests added to internal/controller/conversion_test.go
  may need to be manually adjusted.

This added api/{self._prev_ver}/doc.go to hold the k8s:conversion-gen
marker that points to the new hub.
"""

        git.commit_stage(stage, msg)
