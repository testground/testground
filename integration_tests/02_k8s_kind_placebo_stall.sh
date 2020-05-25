#!/bin/bash

set -o errexit
set -o pipefail
set -e

err_report() {
    echo "Error on line $1"
}

trap 'err_report $LINENO' ERR

function finish {
  kill -15 $DAEMONPID
}
trap finish EXIT

TEMPDIR=`mktemp -d`
testground daemon &
DAEMONPID=$!
testground plan import --from plans/placebo
testground build single --builder docker:go --plan placebo | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:placebo
kind load docker-image testplan:placebo
pushd $TEMPDIR
testground run single --runner cluster:k8s --builder docker:go --use-build testplan:placebo --instances 2 --plan placebo --testcase stall &
sleep 10
BEFORE=$(kubectl get pods | grep placebo | grep Running | wc -l)
testground terminate --runner=cluster:k8s
sleep 5
AFTER=$(kubectl get pods | grep placebo | grep Running | wc -l)
test $BEFORE -gt $AFTER
exit $?
