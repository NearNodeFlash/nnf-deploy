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

# Various system initialization scripts.
# TODO: System Initialization should move to the nnf-deploy tool with configuration defined in ./config/systems.yaml
#       This would therefore guarantee we are using a single source point for all systems.

source common.sh

CMD="$1"

case "$CMD" in
dp0)
    # The following commands apply to initializing the current DP0 environment
    # Nodes containing 'cn' are considered to be worker nodes for the time being.
    COMPUTE_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep cn | paste -d" " -s -)
    RABBIT_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -v cn | grep -v master | paste -d" " -s -)
    MASTER_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep master | paste -d" " -s -)

    # We are using COMPUTE_NODES as generic k8s workers.
    WORKER_NODES="$COMPUTE_NODES"
    ;;

dp1)
    # The following commands apply to initializing the current DP1 environment
    WORKER_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -i 'worker' | paste -d" " -s -)
    RABBIT_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -i 'node'   | grep -v master | paste -d" " -s -)
    MASTER_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -i 'master' | paste -d" " -s -)

    ;;

htx2)
    # The following commands apply to initializing the current HTX-2 environment
    WORKER_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -i 'rabbit-compute-5' | paste -d" " -s -)
    RABBIT_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -i 'rabbit-node-2'    | grep -v master | paste -d" " -s -)
    MASTER_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -i 'rabbit-compute-4' | paste -d" " -s -)

    ;;


craystack)
    # The following commands apply to initializing the current Craystack-lop environment
    RABBIT_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -i 'rabbit' | paste -d" " -s -)
    MASTER_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -i 'master' | paste -d" " -s -)

    # We are using MASTER_NODES as generic k8s workers.
    WORKER_NODES="$MASTER_NODES"
    ;;

*)
    echo "Unknown system: $CMD"
    exit 1
    ;;
esac

echo WORKER_NODES "$WORKER_NODES"
echo RABBIT_NODES "$RABBIT_NODES"
echo MASTER_NODES "$MASTER_NODES"

# Label the WORKER_NODES to allow them to handle wlm and nnf-sos
for NODE in $WORKER_NODES; do
    # Label them for SLCMs.
    kubectl label node "$NODE" cray.nnf.manager=true
    kubectl label node "$NODE" cray.wlm.manager=true
done

for NODE in $RABBIT_NODES; do
    # Taint the rabbit nodes for the NLCMs, to keep any
    # non-NLCM pods off of them.
    kubectl taint node "$NODE" cray.nnf.node=true:NoSchedule

    # Label the rabbit nodes for the NLCMs.
    kubectl label node "$NODE" cray.nnf.node=true
    kubectl label node "$NODE" cray.nnf.x-name="$NODE"
done

#Required for webhooks
install_cert_manager

exit 0

