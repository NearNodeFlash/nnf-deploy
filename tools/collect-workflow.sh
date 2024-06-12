#!/bin/bash

PROG=$(basename "$0")

usage() {
    echo "$PROG: <-d collection_dir> <-w workflow> [-o yaml| -o json]"
    echo
    echo "   -d collection_dir   Directory to create for data collection"
    echo "                       (default uses name of workflow)."
    echo "   -w workflow         Name of workflow."
    echo "   -o yaml|json        Use YAML or JSON output (default json)."
}

FORMAT="json"
FMTTOOL="jq"

while getopts "d:o:w:h" opt; do
    case $opt in
    d) WORKDIR="$OPTARG" ;;
    w) WORKFLOW="$OPTARG" ;;
    o) case "$OPTARG" in
       yaml) FORMAT="yaml"
             FMTTOOL="yq"
             ;;
       *) # use default
          ;;
       esac
       ;;
    h) usage
       exit 0
       ;;
    :) echo "Option -$OPTARG requires an argument"
       exit 1
       ;;
    ?) echo "Invalid option -$OPTARG"
       exit 1
       ;;
    esac
done
shift $((OPTIND-1))

if [[ -z $WORKFLOW ]]; then
    echo "Must specify -w"
    exit 1
fi
if [[ -z $WORKDIR ]]; then
    WORKDIR="$WORKFLOW"
fi

if [[ -e $WORKDIR ]]; then
    echo "The workdir must not exist"
    exit 1
fi
mkdir -p "$WORKDIR" || exit 1
cd "$WORKDIR" || exit 1

echo "Get workflow $WORKFLOW"
wf_file="workflow.$FORMAT"
if ! kubectl get workflow "$WORKFLOW" -o "$FORMAT" > "$wf_file"; then
    echo "Unable to get workflow: $WORKFLOW"
    exit 1
fi

# Get all of the Kinds of resources that are created by a Workflow that is
# doing data movement.
echo "Get all Kinds"
wf_uid=$("$FMTTOOL" -rM .metadata.uid "$wf_file")
kinds="computes,directivebreakdowns,servers,nnfnodestorages,nnfnodeblockstorages,nnfstorages,nnfaccesses,clientmounts"
(
    kubectl get workflow "$WORKFLOW" -o wide
    echo
    kubectl get "$kinds" -A -l "dataworkflowservices.github.io/workflow.uid=$wf_uid" -o wide
) > summary.txt
kubectl get "$kinds" -A -l "dataworkflowservices.github.io/workflow.uid=$wf_uid" -o "$FORMAT" > "all.$FORMAT"

