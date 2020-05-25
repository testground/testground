#!/bin/bash

set -o errexit
set -e

err_report() {
    echo "Error on line $1 $2"
}
FILENAME=`basename $0`
trap 'err_report $LINENO $FILENAME' ERR

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

pushd $TEMPDIR
testground healthcheck --runner local:docker --fix
testground run single --runner local:docker --builder docker:go --use-build testplan:placebo --instances 2 --plan placebo --testcase stall &
sleep 20
BEFORE=$(docker ps | grep placebo | wc -l)
testground terminate --runner=local:docker
sleep 10 
AFTER=$(docker ps | grep placebo | wc -l)
test $BEFORE -gt $AFTER
exit $?
