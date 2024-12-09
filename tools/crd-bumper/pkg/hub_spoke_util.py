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

from .fileutil import FileUtil


class HubSpokeUtil:
    """Simple utilities related to hub and spokes."""

    @staticmethod
    def is_spoke(ver):
        """
        Determine whether or not the 'ver' API is a spoke.
        """
        path = f"api/{ver}/conversion.go"
        if not os.path.isfile(path):
            return False
        fu = FileUtil(False, path)
        line = fu.find_in_file(" ConvertTo(dstRaw conversion.Hub) ")
        if line is None:
            return False
        return True

    @staticmethod
    def is_hub(ver):
        """
        Determine whether or not the 'ver' API is a hub.
        """
        path = f"api/{ver}/conversion.go"
        if not os.path.isfile(path):
            return False
        fu = FileUtil(False, path)
        line = fu.find_in_file(" Hub() ")
        if line is None:
            return False
        return True
