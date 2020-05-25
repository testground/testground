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
testground run single --runner cluster:k8s --builder docker:go --use-build testplan:placebo --instances 1 --plan placebo --testcase ok --collect | tee run.out
RUNID=$(awk '/finished run with ID/ { print $9 }' run.out)
echo "checking run $RUNID"
tar -xzvvf $RUNID.tgz
SIZEOUT=$(cat ./"$RUNID"/single/0/run.out | wc -c)
echo "run.out is $SIZEOUT bytes."
SIZEERR=$(cat ./"$RUNID"/single/0/run.err | wc -c)
popd
test $SIZEOUT -gt 0 && test $SIZEERR -eq 0

exit $?
