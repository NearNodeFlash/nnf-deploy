#!/bin/bash

# Copyright 2021, 2022 Hewlett Packard Enterprise Development LP
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

# Build in each submodule with the submodule defaults. If you wish to use an overlay
# see "hpc-rabsw-nnf-deploy make docker-build"; this will use the overlays defined
# in the ./config/systems.yaml

source common.sh

for SUBMODULE in $SUBMODULES; do
    ( cd $SUBMODULE && make docker-build )
done