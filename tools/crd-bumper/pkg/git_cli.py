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

from .copyright import Copyright

class GitCLI:

  def __init__(self, dryrun, nocommit):
    self._dryrun = dryrun
    if dryrun:
        nocommit = True
    self._nocommit = nocommit

  def verify_branch(self, branch):
    cmd = "git status | sed 1q | awk '{{print $3}}'"
    child = subprocess.Popen(cmd, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    res = child.communicate()
    if child.returncode != 0:
      raise Exception(f"Unable to confirm branch: {res[1]}")
    bname = res[0].strip()
    if bname != self._branch:
      raise Exception(f"Current branch is {bname}, desired branch is {branch}")

  def checkout_branch(self, branch):
    if branch in ['main', 'master', 'releases/v0']:
      raise Exception("Branch name must not be main, master, or releases/v0")
    self.verify_clean()
    cmd = f"git checkout -b {branch}"
    if self._dryrun:
      print(f"Dryrun: {cmd}")
    else:
      print(f"Run: {cmd}")
      child = subprocess.Popen(cmd, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
      res = child.communicate()
      if child.returncode != 0:
        raise Exception(f"Unable to checkout branch {branch}: {res[1]}")

  def mv(self, src, dst):
    cmd = f"git mv {src} {dst}"
    if self._dryrun:
      print(f"Dryrun: {cmd}")
    else:
      print(f"Run: {cmd}")
      mv_child = subprocess.Popen(cmd, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
      res = mv_child.communicate()
      if mv_child.returncode != 0:
         raise Exception(f"Unable to execute command to move code: {mv_child.returncode}, err: {res[1]}")

  def _add_files(self):

    self._update_copyrights()

    cmd = "git add -A"
    if self._nocommit:
      print(f"Dryrun: {cmd}")
    else:
      print(f"Run: {cmd}")
      child = subprocess.Popen(cmd, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
      res = child.communicate()
      if child.returncode != 0:
        raise Exception(f"Unable to add files: {res[1]}")

  def _update_copyrights(self):
    cmd = "git status -s"
    if self._dryrun:
      print(f"Dryrun: {cmd}")
    else:
      print(f"Run: {cmd}")
      child = subprocess.Popen(cmd, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
      res = child.communicate()
      if child.returncode != 0:
        raise Exception(f"Unable to query git status: {res[1]}")
      cright = Copyright(self._dryrun)
      for x in res[0].split("\n"):
        if len(x) == 0:
          break
        fname = x.split()[-1]
        if not fname.startswith("github/"):
          if os.path.isfile(fname):
            cright.update(fname)

  def verify_clean(self):
    cmd = "git status -s"
    if self._dryrun:
      print(f"Dryrun: {cmd}")
    else:
      print(f"Run: {cmd}")
      child = subprocess.Popen(cmd, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
      res = child.communicate()
      if child.returncode != 0:
        raise Exception(f"Unable to verify that workarea is clean: {res[1]}")
      status = res[0].strip()
      if len(status) > 0:
        raise Exception(f"Workarea is not clean: {res[0]}")

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
      child = subprocess.Popen(cmd, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
      res = child.communicate()
      if child.returncode != 0:
        raise Exception(f"Unable to commit: {res[0]}\n{res[1]}")
      self.verify_clean()

  def get_previous(self):
    """
    Get the previous commit operation, if it was one of ours.
    """

    cmd = "git log -1 --format='%B'"
    child = subprocess.Popen(cmd, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    res = child.communicate()
    if child.returncode != 0:
      raise Exception(f"Unable to get log: {res[1]}")
    operation = res[0].strip().split("\n", maxsplit=1)[0]
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
        raise Exception(f"Operation {next_operation} wants to build on {expect_token}, but found that the previous operation was {prev_cmd_token}.")

