#!/bin/bash

# Copyright 2021-2024 Hewlett Packard Enterprise Development LP
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

CMD=$1

function create_cluster {
    CONFIG=kind-config.yaml

    # Rabbit taints/labels, plus some host mounts for data movement
    RABBITCONFIG=$(cat << EOF

  extraMounts:
    - hostPath: /tmp/nnf
      containerPath: /nnf
      propagation: None
EOF
)

    cat > $CONFIG <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "127.0.0.1"
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    apiServer:
        # enable auditing flags on the API server
        extraArgs:
          audit-log-path: /var/log/kubernetes/kube-apiserver-audit.log
          audit-policy-file: /etc/kubernetes/policies/audit-policy.yaml
          audit-log-maxsize: "100"
        # mount new files / directories on the control plane
        extraVolumes:
          - name: audit-policies
            hostPath: /etc/kubernetes/policies
            mountPath: /etc/kubernetes/policies
            readOnly: true
            pathType: "DirectoryOrCreate"
          - name: "audit-logs"
            hostPath: "/var/log/kubernetes"
            mountPath: "/var/log/kubernetes"
            readOnly: false
            pathType: DirectoryOrCreate
  # mount the local file on the control plane
  extraMounts:
  - hostPath: ./config/audit-policy.yaml
    containerPath: /etc/kubernetes/policies/audit-policy.yaml
    readOnly: true
- role: worker
- role: worker $RABBITCONFIG
- role: worker $RABBITCONFIG
EOF

    # create a file for data movement
    if [ ! -f /tmp/nnf/file.in ]; then
        mkdir -p /tmp/nnf && dd if=/dev/zero of=/tmp/nnf/file.in bs=128 count=0 seek=$((1024 * 1024))
    fi

    kind create cluster --wait 60s --image=kindest/node:v1.27.2 --config $CONFIG

    # Use the same init routines that we use on real hardware.
    # This applies taints and labels to rabbit nodes, and installs other
    # services that rabbit software requires.
    ./nnf-deploy init
}

function destroy_cluster {
    kind delete cluster
}

function reset_cluster {
  destroy_cluster
  create_cluster
}

function push_submodules {
  SUBMODULES=$(git submodule status | awk '{print $2}')
  for SUBMODULE in $SUBMODULES; do
    (cd $SUBMODULE && make kind-push)
  done
}

if [[ "$CMD" == "create" ]]; then
  create_cluster
elif [[ "$CMD" == "destroy" ]]; then
  destroy_cluster
elif [[ "$CMD" == "reset" ]]; then
  reset_cluster
elif [[ "$CMD" == "push" ]]; then
  push_submodules
else
  echo "Usage: $0 <create|destroy|reset|push>"
  exit 1
fi
