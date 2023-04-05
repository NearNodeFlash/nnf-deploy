#!/bin/bash

# Copyright 2021-2023 Hewlett Packard Enterprise Development LP
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

install_cert_manager() {
    # Required for webhooks
    certver="v1.7.0"
    kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/"$certver"/cert-manager.yaml
}

install_mpi_operator() {
    # Required for containers
    mpiversion="v0.4.0"
    kubectl apply -f https://raw.githubusercontent.com/kubeflow/mpi-operator/"$mpiversion"/deploy/v2beta1/mpi-operator.yaml
}

SUBMODULES=$(git submodule status | awk '{print $2}')
