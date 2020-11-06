#!/bin/bash

my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

testground plan import --from plans/example-js
testground build single --builder docker:node --plan example-js --wait | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:example-js

pushd $TEMPDIR
testground healthcheck --runner local:docker --fix
testground run single --runner local:docker --builder docker:node --use-build testplan:example-js --instances 2 --plan example-js --testcase pingpong --collect --wait | tee stdout.out
RUNID=$(awk '/finished run with ID/ { print $9 }' stdout.out)
echo "checking run $RUNID"
file $RUNID.tgz
tar -xzvvf $RUNID.tgz
SIZEOUT=$(cat ./"$RUNID"/single/0/run.out | wc -c)
echo "run.out is $SIZEOUT bytes."
SIZEERR=$(cat ./"$RUNID"/single/0/run.err | wc -c)
test $SIZEOUT -gt 0 && test $SIZEERR -eq 0
pushd $RUNID
OUTCOMEOK=$(find . | grep run.out | xargs awk '{print $0, FILENAME}' | grep "\"success_event\"" | wc -l)
test $OUTCOMEOK -eq 2
popd
popd

echo "terminating remaining containers"
testground terminate --runner local:docker
