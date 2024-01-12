#!/bin/bash

# Copyright 2023 Hewlett Packard Enterprise Development LP
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

# Install Prometheus on an existing cluster.

CMD=$1

INSTANCE_NAME=rabbit
LOCAL_NAME=prometheus-community
NAMESPACE=monitoring
CHART_VER=52.1.0

set -e

# We want helm v3.
helm version | grep -qE Version:.v3

if [[ $CMD == 'deploy' ]]; then

    helm repo add $LOCAL_NAME https://prometheus-community.github.io/helm-charts
    helm repo update $LOCAL_NAME

    helm install $INSTANCE_NAME prometheus-community/kube-prometheus-stack \
      --version $CHART_VER --create-namespace --namespace $NAMESPACE \
      --values config/service-monitor-selector.yaml

    helm list -n $NAMESPACE
fi

if [[ $CMD == 'undeploy' ]]; then
    # This does not uninstall the CRDs.
    helm uninstall $INSTANCE_NAME -n $NAMESPACE
fi

exit 0
