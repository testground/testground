#!/bin/bash

my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

testground plan import --from plans/splitbrain
testground build single --builder docker:go --plan splitbrain | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:splitbrain

pushd $TEMPDIR
testground healthcheck --runner local:docker --fix
testground run single --runner local:docker --builder docker:go --use-build testplan:splitbrain --instances 3 --plan splitbrain --testcase accept --collect | tee stdout.out
RUNID=$(awk '/finished run with ID/ { print $9 }' stdout.out)
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
testground terminate --runner local:docker
testground terminate --builder docker:go
