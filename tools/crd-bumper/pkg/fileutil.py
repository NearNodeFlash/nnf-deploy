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


class FileUtil:

    def __init__(self, dryrun, fpath):
        self._dryrun = dryrun
        self._fpath = fpath
        self._input_data = None

    def read(self):
        if self._input_data is None:
            try:
                with open(self._fpath, "r", encoding="utf-8") as f1:
                    self._input_data = f1.read()
            except Exception as ex:
                print(f"{ex}: reading file {self._fpath}")
                raise

    def store(self):
        if self._input_data is not None:
            f2 = open(f"{self._fpath}.new", "w", encoding="utf-8")
            f2.write(self._input_data)
            f2.close()

            if not self._dryrun:
                os.rename(f"{self._fpath}.new", self._fpath)

    def replace_in_file(self, from_str, to_str):
        self.read()
        changed = False
        if self._input_data is not None:
            input_data = self._input_data.replace(from_str, to_str)
            if input_data != self._input_data:
                changed = True
                self._input_data = input_data
                if self._input_data is None:
                    raise Exception("Hey, we lost the input data")
        return changed

    def delete_from_file(self, from_str):
        self.read()
        changed = False
        if self._input_data is not None:
            new_data = ""
            for line in self._input_data.split("\n"):
                if line != from_str:
                    new_data += f"{line}\n"
            if new_data != self._input_data:
                changed = True
                self._input_data = new_data
        return changed

    def find_in_file(self, substr):
        self.read()
        if self._input_data is not None:
            for line in self._input_data.split("\n"):
                if substr in line:
                    return line
        return None

    def find_with_pattern(self, pat):
        self.read()
        if self._input_data is not None:
            for line in self._input_data.split("\n"):
                if re.search(pat, line):
                    return line
        return None

    def append(self, line):
        self.read()
        if self._input_data is None:
            self._input_data = ""
        self._input_data += line