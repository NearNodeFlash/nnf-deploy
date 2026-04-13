#!/bin/bash

# Copyright 2021-2026 Hewlett Packard Enterprise Development LP
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

set -e
set -o pipefail

# This script must be run from the root of the nnf-deploy repo, not from tools/.
if [[ ! -f "tools/kind.sh" ]]; then
    echo "ERROR: kind.sh must be run from the root of the nnf-deploy repo."
    echo "  cd \$(git -C \"\$(dirname \"\$0\")\" rev-parse --show-toplevel) && tools/kind.sh $*"
    exit 1
fi

# kind v0.31.0+ is required for Kubernetes 1.35 support.
KIND_MIN_VERSION="0.31.0"
KIND_VERSION=$(kind version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)
if [[ -z "$KIND_VERSION" ]]; then
    echo "ERROR: kind is not installed or not in PATH."
    exit 1
fi
if ! printf '%s\n%s\n' "$KIND_MIN_VERSION" "$KIND_VERSION" | sort -V -C; then
    echo "ERROR: kind v$KIND_VERSION is installed but v$KIND_MIN_VERSION or later is required for Kubernetes 1.35."
    exit 1
fi

usage() {
    echo "Usage: $0 [--no-argocd] <CMD>"
    echo
    echo "Options:"
    echo "  --no-argocd             Deploy without ArgoCD using the legacy overlay."
    echo "                          Installs cert-manager, mpi-operator, and lustre"
    echo "                          operators directly instead of via ArgoCD."
    echo
    echo "Commands:"
    echo "  create                  Create a new KIND cluster."
    echo "  destroy                 Destroy the existing KIND cluster."
    echo "  reset                   Destroy the existing KIND cluster and create"
    echo "                          a new cluster. Note: locally-built images are"
    echo "                          not automatically reloaded -- re-run 'push' or"
    echo "                          './nnf-deploy make kind-push' after a reset."
    echo "  push                    Execute 'make kind-push' in each submodule dir."
    echo "  argocd_attach <new_password>"
    echo "                          Login to the argocd instance and set the new"
    echo "                          password. Then add the Git repo to the instance."
    echo "  argocd_login <new_password>"
    echo "                          Only do the login to the argocd instance and set"
    echo "                          the password."
    echo "  argocd_add_git_repo     Only add the git repo to the argocd instance."
    echo "  help                    Show this help message."
}

NO_ARGOCD=
while [[ "$1" == --* ]]; do
    case "$1" in
    --no-argocd) NO_ARGOCD=1 ;;
    --help) usage; exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
    esac
    shift
done

CMD="$1"
shift

function clear_mock_fs {
    if [[ -d /tmp/nnf ]]; then
        rm -rf /tmp/nnf
        mkdir /tmp/nnf
    fi
}

function inject_ca_certs {
    local certs="$KIND_CA_CERTS"
    local tmp_certs=""

    # If KIND_CA_CERTS is not set, auto-extract system CA certs.
    if [[ -z "$certs" ]]; then
        tmp_certs=$(mktemp /tmp/kind-ca-certs.XXXXXX.pem)
        if [[ "$(uname)" == "Darwin" ]]; then
            if [[ -f /Library/Keychains/System.keychain ]]; then
                security find-certificate -a -p /Library/Keychains/System.keychain > "$tmp_certs" 2>/dev/null
            fi
        else
            # Linux: copy the system CA bundle.
            if [[ -f /etc/ssl/certs/ca-certificates.crt ]]; then
                cp /etc/ssl/certs/ca-certificates.crt "$tmp_certs"
            elif [[ -f /etc/pki/tls/certs/ca-bundle.crt ]]; then
                cp /etc/pki/tls/certs/ca-bundle.crt "$tmp_certs"
            fi
        fi

        if [[ ! -s "$tmp_certs" ]]; then
            echo "WARNING: Could not extract system CA certificates. Set KIND_CA_CERTS to a PEM file if image pulls fail with TLS errors."
            rm -f "$tmp_certs"
            return
        fi
        certs="$tmp_certs"
    fi

    if [[ ! -f "$certs" ]]; then
        echo "ERROR: KIND_CA_CERTS file not found: $certs"
        return 1
    fi

    echo "Injecting CA certificates from $certs into KIND nodes..."
    for node in $(kind get nodes); do
        docker cp "$certs" "$node:/usr/local/share/ca-certificates/extra-ca-certs.crt"
        docker exec "$node" update-ca-certificates
        docker exec "$node" systemctl restart containerd
        echo "  $node: done"
    done

    # Clean up temp file if we created one.
    [[ -n "$tmp_certs" ]] && rm -f "$tmp_certs"
}

# Create the integration test user on KIND Rabbit worker nodes.
# The nnf-integration-test suite verifies that UID/GID exist on Rabbit nodes
# (defaults: UID=1051 GID=1052, the flux user). Without this, the BeforeSuite
# check fails and no tests run.
function create_test_user {
    local uid=${NNF_USER_ID:-1051}
    local gid=${NNF_GROUP_ID:-1052}

    echo "Creating test user (UID=$uid, GID=$gid) on KIND Rabbit worker nodes..."
    for node in $(kind get nodes | grep -v control-plane); do
        # Create group and user if they don't already exist
        docker exec "$node" sh -c "getent group $gid >/dev/null 2>&1 || groupadd -g $gid testgroup"
        docker exec "$node" sh -c "getent passwd $uid >/dev/null 2>&1 || useradd -u $uid -g $gid -M -s /bin/bash testuser"
    done
}

function create_cluster {
    CONFIG=kind-config.yaml

    # Rabbit taints/labels, plus some host mounts for data movement
    RABBITCONFIG=$(cat << EOF

  extraMounts:
    - hostPath: /tmp/nnf
      containerPath: /mnt/nnf
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
    apiVersion: kubeadm.k8s.io/v1beta3
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

    # Clear any earlier mock filesystem environment.
    clear_mock_fs
    # create a file for data movement
    if [ ! -f /tmp/nnf/file.in ]; then
        mkdir -p /tmp/nnf && dd if=/dev/zero of=/tmp/nnf/file.in bs=128 count=0 seek=$((1024 * 1024))
    fi

    kind create cluster --wait 60s --image=kindest/node:v1.35.0@sha256:452d707d4862f52530247495d180205e029056831160e22870e37e3f6c1ac31f --config $CONFIG

    # If corporate/custom CA certificates are available, inject them into
    # each KIND node so containerd can pull from registries behind a TLS
    # intercepting proxy. Set KIND_CA_CERTS to a PEM file path, or it
    # defaults to /tmp/corporate-certs.pem.
    inject_ca_certs

    # Use the same init routines that we use on real hardware.
    # This applies taints and labels to rabbit nodes, and installs other
    # services that rabbit software requires.
    if [[ -n $NO_ARGOCD ]]; then
        echo "Deploying without ArgoCD (legacy overlay mode)..."
        cp config/overlay-legacy.yaml-template overlay-legacy.yaml
    fi
    ./nnf-deploy init

    # Create the test user on Rabbit nodes for integration tests
    create_test_user
}

function have_argocd {
    if which helm > /dev/null 2>&1; then
        chart_instance=$(helm list -n argocd --deployed -o json 2>/dev/null | jq -rM '.[0]|select(.chart|test("argo-cd-")).name' 2>/dev/null)
        echo "$chart_instance"
    fi
}

function argocd_login {
    # Set the ArgoCD admin password, and login as the admin user, and add the
    # gitops repo.
    local NEWPW="$1"

    chart_instance=$(have_argocd)
    [[ -z $chart_instance ]] && return

    if [[ -z $NEWPW ]]; then
        echo "You must specify the new password to set on the admin account."
        exit 1
    fi

    for dep in argocd-server argocd-repo-server argocd-applicationset-controller
    do
        kubectl wait deploy -n argocd --timeout=180s "$chart_instance-$dep" --for jsonpath='{.status.availableReplicas}=1'
    done

    dep=argocd-application-controller
    kubectl wait statefulset -n argocd --timeout=180s "$chart_instance-$dep" --for jsonpath='{.status.availableReplicas}=1'

    export ARGOCD_OPTS='--port-forward --port-forward-namespace argocd'
    initialPW=$(argocd admin initial-password -n argocd | sed 1q)
    argocd login --plaintext 127.0.0.1:8080 --username admin --password "$initialPW"
    argocd account update-password --new-password "$NEWPW" --current-password "$initialPW"

    echo
    echo "You are now logged in to ArgoCD as the admin user."
    echo
}

function argocd_add_git_repo {
    # Add your personal gitops repo to the ArgoCD environment. This requires
    # that you have already logged in to the ArgoCD instance.
    #
    # Required environment variables:
    #
    # GH_PERSONAL_GITOPS_REPO           The URL to your personal gitops repo.
    #   Example: https://github.com/roehrich-hpe/gitops.git
    #
    # GH_TOKEN_PERSONAL_GITOPS          A read-only Git token for your repo.
    #   See: https://github.com/NearNodeFlash/argocd-boilerplate?tab=readme-ov-file#using-with-kind-or-a-private-repo
    #
    # GH_USER                           Your Git user name.
    #   Example: roehrich-hpe

    chart_instance=$(have_argocd)
    [[ -z $chart_instance ]] && return

    if [[ -n $GH_PERSONAL_GITOPS_REPO && -n $GH_USER && -n $GH_TOKEN_PERSONAL_GITOPS ]]; then
        export ARGOCD_OPTS='--port-forward --port-forward-namespace argocd'
        argocd repo add "$GH_PERSONAL_GITOPS_REPO" --username "$GH_USER" --password "$GH_TOKEN_PERSONAL_GITOPS" --name gitops-kind
    else
        echo "Unable to add gitops repo. Missing one of: GH_PERSONAL_GITOPS_REPO, GH_USER, GH_TOKEN_PERSONAL_GITOPS"
        echo "Supply those, then re-run: $0 argocd_add_git_repo"
    fi
}

function destroy_cluster {
    kind delete cluster
    clear_mock_fs
}

function reset_cluster {
    destroy_cluster
    create_cluster
}

function push_submodules {
    SUBMODULES=$(git submodule status | awk '{print $2}')
    for SUBMODULE in $SUBMODULES; do
        (cd "$SUBMODULE" && make kind-push)
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
elif [[ "$CMD" == "argocd_attach" ]]; then
    argocd_login "$*"
    argocd_add_git_repo
elif [[ "$CMD" == "argocd_login" ]]; then
    argocd_login "$*"
elif [[ "$CMD" == "argocd_add_git_repo" ]]; then
    argocd_add_git_repo
elif [[ "$CMD" == "help" || "$CMD" == "--help" ]]; then
    usage
else
    usage
    exit 1
fi
