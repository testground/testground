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
testground run single --runner local:exec --builder exec:go --instances 2 --plan placebo --testcase ok --collect | tee run.out
RUNID=$(awk '/finished run with ID/ { print $9 }' run.out)
echo "checking run $RUNID"
tar -xzvvf $RUNID.tgz
SIZEOUT=$(cat ./"$RUNID"/single/0/run.out | wc -c)
echo "run.out is $SIZEOUT bytes."
SIZEERR=$(cat ./"$RUNID"/single/0/run.err | wc -c)
test $SIZEOUT -gt 0 && test $SIZEERR -eq 0

exit $?
