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
import yaml

from .fileutil import FileUtil

class Project:

  def __init__(self, dryrun, project_file=None):
    self._dryrun = dryrun
    self._fname = "PROJECT" if project_file is None else project_file
    self._comment = ""

    # Preserve the introductory comments from kubebuilder.
    fu = FileUtil(self._dryrun, self._fname)
    fu.read()
    for line in fu._input_data.split('\n'):
      if line.startswith('#'):
        self._comment += line + "\n"
      else:
        break

    # Read the YAML
    with open(self._fname, "r", encoding='utf-8') as stream:
        self._yaml = yaml.safe_load(stream)

  def store(self):
    newname = f"{self._fname}.new"
    with open(newname, "w", encoding='utf-8') as stream:
      if self._comment != "":
        stream.write(self._comment)
      yaml.dump(self._yaml, stream)
    if not self._dryrun:
      os.rename(newname, self._fname)

  def kinds(self, version):
    """Return a list containing the names of all kinds that have the given
    API version.
    The list will be sorted so that multiple calls can be compared.
    """
    kinds = []
    for entry in self._yaml['resources']:
      if 'api' in entry:
        if entry['version'] == version:
          kinds.append(entry['kind'])
    kinds.sort()
    return kinds

  def controllers_with_nonlocal_api(self):
    """Return a list containing the names of all controller-only entries that
    use APIs that don't live here.

    The list will be sorted so that multiple calls can be compared.
    """
    kinds = []
    for entry in self._yaml['resources']:
      if 'api' not in entry:
        kinds.append(entry['kind'])
    kinds.sort()
    return kinds

  def name(self):
    return self._yaml['projectName']

  def has_webhooks(self, kind, version):
    idx = self.find_idx(kind, version)
    return True if 'webhooks' in self._yaml['resources'][idx] else False

  def group(self, kind, version):
    """Return the group for the "kind" at the given API version."""
    idx = self.find_idx(kind, version)
    return self._yaml['resources'][idx]['group']

  def find_idx(self, kind, version):
    """Return the index of the "kind" at the given API version."""
    for i in range(len(self._yaml['resources'])):
      if self._yaml['resources'][i]['kind'] == kind and self._yaml['resources'][i]['version'] == version:
        return i
    raise Exception(f"unable to find resource API: {kind}.{version} in {self._fname}")

  def mv_webhooks(self, kind, from_ver, to_ver):
    """Move the "webhooks" section from one API version to the other, for the
    given KIND.
    """
    from_idx = self.find_idx(kind, from_ver)
    to_idx = self.find_idx(kind, to_ver)
    if 'webhooks' in self._yaml['resources'][from_idx]:
      self._yaml['resources'][to_idx]['webhooks'] = self._yaml['resources'][from_idx]['webhooks']
      del self._yaml['resources'][from_idx]['webhooks']

