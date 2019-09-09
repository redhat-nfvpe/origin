#!/bin/bash
# Original script found at: https://github.com/morvencao/kube-mutating-webhook-tutorial/blob/master/deployment/webhook-patch-ca-bundle.sh

ROOT=$(cd $(dirname $0)/../../; pwd)

set -o errexit
set -o nounset
set -o pipefail

export CA_BUNDLE=$(oc get configmap openshift-service-ca -n kube-system -o=jsonpath='{.data.service-ca\.crt}' | base64 | tr -d '\n')

if command -v envsubst >/dev/null 2>&1; then
    envsubst
else
    sed -e "s|\${CA_BUNDLE}|${CA_BUNDLE}|g"
fi
