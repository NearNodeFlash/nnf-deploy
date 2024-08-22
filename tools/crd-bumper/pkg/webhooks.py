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
import sys

from .project import Project
from .fileutil import FileUtil

class MvWebhooks:

  def __init__(self, dryrun, project, prev_ver, new_ver, preferred_alias, module):
    if not isinstance(project, Project):
      raise Exception("need a Project")
    self._dryrun = dryrun
    self._project = project
    self._prev_ver = prev_ver
    self._new_ver = new_ver
    self._preferred_alias = preferred_alias
    self._module = module

  def edit_go_files(self):
    for root, _, f_names in os.walk(f"api/{self._new_ver}", followlinks=False):
      for fname in f_names:
        if 'webhook' in fname:
          fu = FileUtil(self._dryrun, os.path.join(root, fname))
          # Pick through this carefully so we don't mess with
          # admissionReviewVersions.
          fu.replace_in_file(f"-{self._prev_ver}-", f"-{self._new_ver}-")
          fu.replace_in_file(f"versions={self._prev_ver},", f"versions={self._new_ver},")
          fu.store()

    # Handle SetupWebhookWithManager and other things in main.go.
    main_name = "cmd/main.go"
    kinds = self._project.kinds(self._prev_ver)
    fu = FileUtil(self._dryrun, main_name)
    for k in kinds:
      if self._preferred_alias is None:
        group = self._project.group(k, self._prev_ver)
      else:
        group = self._preferred_alias
      # This matches: dwsv1alpha1.Workflow
      fu.replace_in_file(f"{group}{self._prev_ver}.{k}", f"{group}{self._new_ver}.{k}")
    fu.store()

  def edit_manifests(self):
    kinds = self._project.kinds(self._prev_ver)
    fu = FileUtil(self._dryrun, "config/webhook/manifests.yaml")
    for k in kinds:
      fu.replace_in_file(f"- {self._prev_ver}", f"- {self._new_ver}")
      fu.replace_in_file(f"-{self._prev_ver}-{k.lower()}", f"-{self._new_ver}-{k.lower()}")
    fu.store()

  def mv_project_webhooks(self):

    kinds = self._project.kinds(self._prev_ver)
    kinds_new_ver = self._project.kinds(self._new_ver)
    if kinds != kinds_new_ver:
      print(f"The {self._prev_ver} list of types does not match the {self._new_ver} list")
      print(f"Kinds prev_ver: {kinds}")
      print(f"Kinds new_ver: {kinds_new_ver}")
      sys.exit(1)

    for k in kinds:
      self._project.mv_webhooks(k, self._prev_ver, self._new_ver)
    self._project.store()

  def commit(self, git, stage):
    """
    Create a commit with a message about relocating the webhooks.
    """

    msg = f"""Move the existing webhooks from {self._prev_ver} to {self._new_ver}."""

    git.commit_stage(stage, msg)

class ConversionWebhooks:

  def __init__(self, dryrun, project, prev_ver, new_ver, preferred_alias, module):
    if not isinstance(project, Project):
      raise Exception("need a Project")
    self._dryrun = dryrun
    self._project = project
    self._prev_ver = prev_ver
    self._new_ver = new_ver
    self._preferred_alias = preferred_alias
    self._module = module

  def create(self):

    kinds = self._project.kinds(self._prev_ver)

    new_webhook_test = False
    for k in kinds:
      if not self._project.has_webhooks(k, self._new_ver):
        group = self._project.group(k, self._new_ver)
        cmd = f"kubebuilder create webhook --group {group} --version {self._new_ver} --kind {k} --conversion"

        if self._dryrun:
          print(f"Dryrun: {cmd}")
          new_webhook_test = True
        else:
          print(f"Running: {cmd}")
          cr_child = subprocess.Popen(shlex.split(cmd), shell=False, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
          res = cr_child.communicate()
          if cr_child.returncode != 0:
            print(f"Error: {res[1]}")
            raise Exception(f"Unable to create webhook for {k}.{self._new_ver}")
          else:
            new_webhook_test = True

          if new_webhook_test:
            self._update_suite_test(k)
            self._edit_test_comment(k)

    return new_webhook_test

  def hub(self):

    hub_file = f"api/{self._new_ver}/conversion.go"
    if not os.path.isfile(hub_file):
      shutil.copy2("hack/boilerplate.go.txt", hub_file, follow_symlinks=False)

      fu = FileUtil(self._dryrun, hub_file)
      fu.append(f"\npackage {self._new_ver}\n\n")
      kinds = self._project.kinds(self._new_ver)
      for k in kinds:
        fu.append(f"func (*{k}) Hub() {{}}\n")

      fu.append("\n// The conversion-verifier tool wants these...though they're never used.\n")
      for k in kinds:
        fu.append(f"func (*{k}List) Hub() {{}}\n")

      fu.store()

  def enable_in_crd(self):

    kustomization = "config/crd/kustomization.yaml"
    fu = FileUtil(self._dryrun, kustomization)
    kinds = self._project.kinds(self._new_ver)
    for uk in kinds:
      k = uk.lower()
      base = f"- path: patches/webhook_in_{k}"
      line = fu.find_with_pattern(f"^{base}")
      if line is None:
        line = fu.find_with_pattern(f"^#{base}")
        if line is not None:
          fu.replace_in_file(line, line[1:])

      base = f"- path: patches/cainjection_in_{k}"
      line = fu.find_with_pattern(f"^{base}")
      if line is None:
        line = fu.find_with_pattern(f"^#{base}")
        if line is not None:
          fu.replace_in_file(line, line[1:])

    fu.store()

  def _edit_test_comment(self, kind):
    """
    Edit new api/<ver>/*_webhook_test.go files that were created by
    kubebuilder. Add our comment to the top explaining the context for the
    test.
    """

    klower = kind.lower()
    test = f"api/{self._new_ver}/{klower}_webhook_test.go"
    fu = FileUtil(self._dryrun, test)
    # Look for the Context() string written by kubebuilder:
    has_intro = fu.find_in_file(f"""Context("When creating {kind} under Conversion Webhook""")
    if has_intro is None:
      # Skip it. We don't have a reference we can use to place the comment.
      return

    # Wake up!  Multi-line f-string:
    template = f'''
	// We already have api/<spoke_ver>/conversion_test.go that is
	// digging deep into the conversion routines, and we have
	// internal/controllers/conversion_test.go that is verifing that the
	// conversion webhook is hooked up to those routines.
''' # END multi-line f-string.

    fu.replace_in_file(has_intro, f"{template}\n{has_intro}")
    fu.store()

  def _update_suite_test(self, kind):
    """
    Add a new SetupWebhookWithManager() to internal/controller/suite_test.go
    for this new Kind, to enable its conversion webhook in the suite_test.

    Don't edit anything else in suite_test.go at this time--the other edits
    will be handled when the entire internal/controller/ dir is updated to
    point at the new hub.
    """

    path = "internal/controller/suite_test.go"
    fu = FileUtil(self._dryrun, path)

    if self._preferred_alias is None:
      group = self._project.group(kind, self._new_ver)
    else:
      group = self._preferred_alias

    scaffold_builder = fu.find_in_file("crdbumper:scaffold:builder")
    new = f"\terr = (&{group}{self._new_ver}.{kind}{{}}).SetupWebhookWithManager(k8sManager)\n\tExpect(err).ToNot(HaveOccurred())"
    fu.replace_in_file(scaffold_builder, f"{new}\n\n{scaffold_builder}")

    fu.store()

  def add_fuzz_tests(self):
    """
    Add basic fuzz tests to
    github/cluster-api/util/conversion/conversion_test.go
    for any Kind that is not yet represented there.

    Don't edit anything else in conversion_test.go at this time--the other
    edits will be handled when the entire github/cluster-api/util/ dir is
    updated to point at the new hub.

    WARNING WARNING: This contains a series of multi-line Python f-strings
    which contain large chunks of Go code. So it's confusing. Stay sharp.
    """

    conv_go = "github/cluster-api/util/conversion/conversion_test.go"
    fu = FileUtil(self._dryrun, conv_go)

    kinds = self._project.kinds(self._new_ver)
    for kind in kinds:
      if self._preferred_alias is None:
        group = self._project.group(kind, self._new_ver)
      else:
        group = self._preferred_alias

      already_done = fu.find_in_file(f"old{kind}GVK")
      if already_done:
        continue

      scaffold_gvk = fu.find_in_file("+crdbumper:scaffold:gvk")
      # Wake up!  Multi-line f-string:
      template = f'''
	old{kind}GVK = schema.GroupVersionKind{{
		Group:   {group}{self._new_ver}.GroupVersion.Group,
		Version: "v1old",
		Kind:    "{kind}",
	}}
''' # END multi-line f-string.
      fu.replace_in_file(scaffold_gvk, f"{template}\n{scaffold_gvk}")

      scaffold_marshal = fu.find_in_file("+crdbumper:scaffold:marshaldata")
      # Wake up!  Multi-line f-string:
      template = f'''
	t.Run("{kind} should write source object to destination", func(*testing.T) {{
		src := &{group}{self._new_ver}.{kind}{{
			ObjectMeta: metav1.ObjectMeta{{
				Name: "test-1",
				Labels: map[string]string{{
					"label1": "",
				}},
			}},
			//Spec: {group}{self._new_ver}.{kind}Spec{{
			//	// ACTION: Fill in a few valid fields so
			//	// they can be tested in the annotation checks
			//	// below.
			//}},
		}}

		dst := &unstructured.Unstructured{{}}
		dst.SetGroupVersionKind(old{kind}GVK)
		dst.SetName("test-1")

		g.Expect(MarshalData(src, dst)).To(Succeed())
		// ensure the src object is not modified
		g.Expect(src.GetLabels()).ToNot(BeEmpty())

		g.Expect(dst.GetAnnotations()[DataAnnotation]).ToNot(BeEmpty())

		// ACTION: Fill in a few valid fields above in the Spec so
		// they can be tested here in the annotation checks.

		//g.Expect(dst.GetAnnotations()[DataAnnotation]).To(ContainSubstring("mgsNids"))
		//g.Expect(dst.GetAnnotations()[DataAnnotation]).To(ContainSubstring("rabbit-03@tcp"))
		//g.Expect(dst.GetAnnotations()[DataAnnotation]).To(ContainSubstring("mountRoot"))
		//g.Expect(dst.GetAnnotations()[DataAnnotation]).To(ContainSubstring("/lus/w0"))
	}})

	t.Run("{kind} should append the annotation", func(*testing.T) {{
		src := &{group}{self._new_ver}.{kind}{{
			ObjectMeta: metav1.ObjectMeta{{
				Name: "test-1",
			}},
		}}
		dst := &unstructured.Unstructured{{}}
		dst.SetGroupVersionKind({group}{self._new_ver}.GroupVersion.WithKind("{kind}"))
		dst.SetName("test-1")
		dst.SetAnnotations(map[string]string{{
			"annotation": "1",
		}})

		g.Expect(MarshalData(src, dst)).To(Succeed())
		g.Expect(dst.GetAnnotations()).To(HaveLen(2))
	}})
''' # END multi-line f-string.
      fu.replace_in_file(scaffold_marshal, f"{template}\n{scaffold_marshal}")

      # Define a serialized annotation here, outside the f-string, because
      # f-strings do not allow backwhacks.
      data_annotation = r'{\"metadata\":{\"name\":\"test-1\",\"creationTimestamp\":null,\"labels\":{\"label1\":\"\"}},\"spec\":{},\"status\":{}}'

      scaffold_unmarshal = fu.find_in_file("+crdbumper:scaffold:unmarshaldata")
      # Wake up!  Multi-line f-string:
      template = f'''
	t.Run("{kind} should return false without errors if annotation doesn't exist", func(*testing.T) {{
		src := &{group}{self._new_ver}.{kind}{{
			ObjectMeta: metav1.ObjectMeta{{
				Name: "test-1",
			}},
		}}
		dst := &unstructured.Unstructured{{}}
		dst.SetGroupVersionKind(old{kind}GVK)
		dst.SetName("test-1")

		ok, err := UnmarshalData(src, dst)
		g.Expect(ok).To(BeFalse())
		g.Expect(err).ToNot(HaveOccurred())
	}})

	t.Run("{kind} should return true when a valid annotation with data exists", func(*testing.T) {{
		src := &unstructured.Unstructured{{}}
		src.SetGroupVersionKind(old{kind}GVK)
		src.SetName("test-1")
		src.SetAnnotations(map[string]string{{
			DataAnnotation: "{data_annotation}",
		}})

		dst := &{group}{self._new_ver}.{kind}{{
			ObjectMeta: metav1.ObjectMeta{{
				Name: "test-1",
			}},
		}}

		ok, err := UnmarshalData(src, dst)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(ok).To(BeTrue())

		g.Expect(dst.GetLabels()).To(HaveLen(1))
		g.Expect(dst.GetName()).To(Equal("test-1"))
		g.Expect(dst.GetLabels()).To(HaveKeyWithValue("label1", ""))
		g.Expect(dst.GetAnnotations()).To(BeEmpty())
	}})

	t.Run("{kind} should clean the annotation on successful unmarshal", func(*testing.T) {{
		src := &unstructured.Unstructured{{}}
		src.SetGroupVersionKind(old{kind}GVK)
		src.SetName("test-1")
		src.SetAnnotations(map[string]string{{
			"annotation-1": "",
			DataAnnotation: "{data_annotation}",
		}})

		dst := &{group}{self._new_ver}.{kind}{{
			ObjectMeta: metav1.ObjectMeta{{
				Name: "test-1",
			}},
		}}

		ok, err := UnmarshalData(src, dst)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(ok).To(BeTrue())

		g.Expect(src.GetAnnotations()).ToNot(HaveKey(DataAnnotation))
		g.Expect(src.GetAnnotations()).To(HaveLen(1))
	}})
''' # END multi-line f-string.
      fu.replace_in_file(scaffold_unmarshal, template + "\n" + scaffold_unmarshal)

      fu.store()

  def commit(self, git, stage):
    """
    Create a commit with a message about relocating the webhooks.
    """

    msg = f"""Create conversion webhooks and hub routines for {self._new_ver}.

This may have used \"kubebuilder create webhook --conversion\" for any
API that did not already have a webhook.

Any newly-created api/{self._new_ver}/*_webhook_test.go is empty and
does not need content at this time. It has been updated with a comment
to explain where conversion tests are located.

ACTION: Any new tests added to
  github/cluster-api/util/conversion/conversion_test.go
  may need to be manually adjusted. Look for the "ACTION" comments
  in this file.

This may have added a new SetupWebhookWithManager() to suite_test.go,
though a later step will complete the changes to that file."""

    git.commit_stage(stage, msg)

