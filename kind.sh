#!/bin/bash

# Various Kubernetes in Docker (KinD) related scripts

source common.sh

CMD=$1

if [ $# -eq 0 ]; then
    echo "One of create, destroy, reset, or push is required"
fi

if [[ "$CMD" == "create" ]]; then
    CONFIG=kind-config.yaml
    cat > $CONFIG << EOF
# three node (two workers) cluster config
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "127.0.0.1"
nodes:
- role: control-plane
- role: worker
  extraMounts:
  - hostPath: /tmp/nnf
    containerPath: /nnf
    propagation: None
- role: worker
  extraMounts:
  - hostPath: /tmp/nnf
    containerPath: /nnf
    propagation: None
EOF

    if [ ! -f /tmp/nnf/file.in ]; then
        mkdir -p /tmp/nnf && dd if=/dev/zero of=/tmp/nnf/file.in bs=128 count=0 seek=$((1024 * 1024))
    fi

    kind create cluster --wait 60s --image=kindest/node:v1.22.5 --config kind-config.yaml

    # TODO: Initialization should move to init.sh
    
    # Use the kind-control-plane node for the SLCMs.  Remove its default taint
    # and label it for our use.
    kubectl taint node kind-control-plane node-role.kubernetes.io/master:NoSchedule-
    kubectl label node kind-control-plane cray.nnf.manager=true
    kubectl label node kind-control-plane cray.wlm.manager=true

    # Taint the kind workers as rabbit nodes for the NLCMs, to keep any
    # non-NLCM pods off of them.
    NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -v control-plane | paste -d" " -s -)
    # shellcheck disable=2086
    kubectl taint nodes $NODES cray.nnf.node=true:NoSchedule

    # Label the kind-workers as rabbit nodes for the NLCMs.
    for NODE in $(kubectl get nodes --no-headers | grep --invert-match "control-plane" | awk '{print $1}'); do
        kubectl label node "$NODE" cray.nnf.node=true
        kubectl label node "$NODE" cray.nnf.x-name="$NODE"
    done

    #Required for webhooks
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
