#!/bin/bash

# Create various resources

CMD=$1

if [[ "$CMD" == create ]]; then
  echo "$(tput bold)Installing sample LustreFileSystem $(tput sgr 0)"
  cat <<-EOF | kubectl apply -f -
apiVersion: cray.hpe.com/v1alpha1
kind: LustreFileSystem
metadata:
  name: lustrefilesystem-sample-maui
  namespace: nnf-dm-system
spec:
  name: maui
  mgsNid: 172.0.0.1@tcp
  mountRoot: /lus/maui
EOF
fi