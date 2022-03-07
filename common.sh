#!/bin/bash

install_cert_manager ()
{
    certver="v1.7.0"
    #Required for webhooks
    kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/"$certver"/cert-manager.yaml
}

SUBMODULES=$(ls -d */ | grep -e "hpc-dpm-" -e "hpc-rabsw-")