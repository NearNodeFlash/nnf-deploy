#!/bin/bash

# Various system initialization scripts.
# TODO: System Initialization should move to the nnf-deploy tool with configuration defined in ./config/systems.yaml
#       This would therefore guarantee we are using a single source point for all systems.

source common.sh

CMD=$1

if [ $# -eq 0 ]; then
    echo "One of dp0 or dp1 is required"
fi


# The following commands apply to initializing the current DP0 environment
# Nodes containing 'cn' are considered to be worker nodes for the time being.
if [[ "$CMD" == dp0 ]]; then
    COMPUTE_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep cn | paste -d" " -s -)
    RABBIT_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -v cn | grep -v master | paste -d" " -s -)
    MASTER_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep master | paste -d" " -s -)

    echo COMPUTE_NODES "$COMPUTE_NODES"
    echo RABBIT_NODES "$RABBIT_NODES"
    echo MASTER_NODES "$MASTER_NODES"

    # Label the COMPUTE_NODES to allow them to handle wlm and nnf-sos
    # We are using COMPUTE_NODES as generic k8s workers
    for NODE in $COMPUTE_NODES; do
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

    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: nnf-node-map
  namespace: default
data:
  rabbit-dev-01: rabbit-dev-cn-01;rabbit-dev-cn-02;rabbit-dev-cn-03;rabbit-dev-cn-04;np05;np06;np07;np08;np09;np10;np11;np12;np13;np14;np15;np16;
EOF

fi

# The following commands apply to initializing the current DP1 environment
# Nodes containing 'cn' are considered to be worker nodes for the time being.
if [[ "$CMD" == dp1 ]]; then
    WORKER_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -i 'worker' | paste -d" " -s -)
    RABBIT_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -i 'node'   | grep -v master | paste -d" " -s -)
    MASTER_NODES=$(kubectl get nodes --no-headers -o custom-columns=:metadata.name | grep -i 'master' | paste -d" " -s -)

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

    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: nnf-node-map
  namespace: default
data:
  rabbit-node-0: rabbit-cn-00-01;np02;np03;np04;np05;np06;np07;np08;np09;np10;np11;np12;np13;np14;np15;np16
  rabbit-node-1: np17;np18;np19;np20;np21;np22;np23;np24;np25;np26;np27;np28;np29;np30;np31;np32
EOF

fi