#!/bin/bash

my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

pushd $TEMPDIR
testground plan import --from plans/placebo
testground run single --runner local:exec --builder exec:go --instances 2 --plan placebo --testcase ok --collect | tee run.out
RUNID=$(awk '/finished run with ID/ { print $9 }' run.out)
echo "checking run $RUNID"
file $RUNID.tgz
LENGTH=${#RUNID}
test $LENGTH -eq 12
tar -xzvvf $RUNID.tgz
SIZEOUT=$(cat ./"$RUNID"/single/0/run.out | wc -c)
echo "run.out is $SIZEOUT bytes."
SIZEERR=$(cat ./"$RUNID"/single/0/run.err | wc -c)
test $SIZEOUT -gt 0 && test $SIZEERR -eq 0
popd

echo "terminating remaining containers"
testground terminate --runner local:exec
