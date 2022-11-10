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

# Various Kubernetes in Docker (KinD) related scripts

source common.sh

CMD=$1

if [ $# -eq 0 ]; then
    echo "One of create, destroy, reset, or push is required"
fi

if [[ "$CMD" == "create" ]]; then
    CONFIG=kind-config.yaml

    # Only write the config if it's not present; this allows customization 
    if ! [[ -f "$CONFIG" ]]; then

      # Rabbit & WLM System Local Controllers (SLC)
      SLCCONFIG=$(cat << EOF

  kubeadmConfigPatches:
  - |
    kind: JoinConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: cray.nnf.manager=true,cray.wlm.manager=true
EOF
)

      # Rabbit taints/labels, plus some host mounts for data movement
      RABBITCONFIG=$(cat << EOF

  extraMounts:
    - hostPath: /tmp/nnf
      containerPath: /nnf
      propagation: None
  kubeadmConfigPatches:
  - |
    kind: JoinConfiguration
    nodeRegistration:
      taints:
      - key: cray.nnf.node
        value: "true"
        effect: NoSchedule
      kubeletExtraArgs:
        node-labels: "cray.nnf.node=true"
EOF
)

      cat > $CONFIG <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "127.0.0.1"
nodes:
- role: control-plane
- role: worker $SLCCONFIG
- role: worker $RABBITCONFIG
- role: worker $RABBITCONFIG
EOF

    fi

    # create a file for data movement
    if [ ! -f /tmp/nnf/file.in ]; then
        mkdir -p /tmp/nnf && dd if=/dev/zero of=/tmp/nnf/file.in bs=128 count=0 seek=$((1024 * 1024))
    fi

    kind create cluster --wait 60s --image=kindest/node:v1.25.2 --config $CONFIG

    # Required for webhooks
    install_cert_manager 
fi

if [[ "$CMD" == destroy ]]; then
    kind delete cluster
fi

if [[ "$CMD" == reset ]]; then
    ./kind.sh destroy
    ./kind.sh create
fi

if [[ "$CMD" == push ]]; then
  for SUBMODULE in $SUBMODULES; do
    (cd $SUBMODULE && make kind-push)
  done
fi
