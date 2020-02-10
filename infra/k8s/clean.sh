#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset

START_TIME=`date +%s`

echo "Resetting Testground environment..."
echo

kubectl delete pods -l app=testground --force --grace-period=0
kubectl delete pods -l name=testground-sidecar --force --grace-period=0
echo

sleep 5

echo "Wait for Sidecar to be Ready..."
echo
WORKER_NODES=$(($(kubectl get nodes | grep -v NotReady | grep Ready | wc -l) - 1))
RUNNING_SIDECARS=0
while [ "$RUNNING_SIDECARS" -ne "$WORKER_NODES" ]; do RUNNING_SIDECARS=$(kubectl get pods | grep testground-sidecar | grep Running | wc -l || true); echo "Got $RUNNING_SIDECARS running sidecar pods"; sleep 5; done;

echo "Testground cluster is ready"
echo

END_TIME=`date +%s`
echo "Clean up time was `expr $END_TIME - $START_TIME` seconds"
