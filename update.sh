#!/bin/bash

for SUBMODULE in $(ls -d */ | grep -e "hpc-dpm" -e "hpc-rabsw"); do
    git submodule update $SUBMODULE
done