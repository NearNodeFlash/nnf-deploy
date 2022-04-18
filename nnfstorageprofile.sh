#!/bin/bash

# Create various resources

CMD=$1

if [[ "$CMD" == create ]]; then
  echo "$(tput bold)Installing sample NnfStorageProfile $(tput sgr 0)"
  cat <<EOF | kubectl apply -f -
apiVersion: nnf.cray.hpe.com/v1alpha1
kind: NnfStorageProfile
metadata:
  name: placeholder
  namespace: nnf-system
data:
  default: true
EOF
fi

