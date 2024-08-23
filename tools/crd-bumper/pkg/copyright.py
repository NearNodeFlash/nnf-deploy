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

import datetime
import re

from .fileutil import FileUtil


class Copyright:

    def __init__(self, dryrun):
        self._dryrun = dryrun
        self._year = str(datetime.date.today().year)

    def update(self, path):
        fu = FileUtil(self._dryrun, path)
        line = fu.find_in_file("Copyright")
        if line is None:
            return
        done = False
        if not done:
            # Copyright 2021-2023
            pat = r"Copyright \d\d\d\d-(\d\d\d\d)"
            p = re.search(pat, line)
            if p is not None:
                d = p.group(1)
                line2 = re.sub(d, self._year, line)
                done = True
        if not done:
            # Copyright 2021,2023
            # Copyright 2021, 2023
            pat = r"(Copyright \d\d\d\d)([, ]+\d\d\d\d)"
            p = re.search(pat, line)
            if p is not None:
                d = p.group(1)
                line2 = re.sub(p.group(0), f"{d}-{self._year}", line)
                done = True
        if not done:
            # Copyright 2022
            pat = r"(Copyright (\d\d\d\d))([^-])"
            p = re.search(pat, line)
            if p is not None:
                d = p.group(2)
                if d != self._year:
                    line2 = re.sub(d, f"{d}-{self._year}", line)
                    done = True
        if done:
            fu.replace_in_file(line, line2)
        fu.store()
