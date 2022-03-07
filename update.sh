#!/bin/bash

source common.sh

for SUBMODULE in $SUBMODULES; do
    # https://git-scm.com/book/en/v2/Git-Tools-Submodules:
    # To also initialize, fetch and checkout any nested submodules,
    # you can use the foolproof git submodule update --init --recursive.
    git submodule update --init --recursive --remote $SUBMODULE
done