#!/bin/bash

my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

testground plan import --from plans/network
testground build single --builder docker:go --plan network | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:network

pushd $TEMPDIR
testground healthcheck --runner local:docker --fix
testground run single --runner local:docker --builder docker:go --use-build testplan:network --instances 2 --plan network --testcase ping-pong --collect | tee stdout.out
RUNID=$(awk '/finished run with ID/ { print $9 }' stdout.out)
echo "checking run $RUNID"
file $RUNID.tgz
tar -xzvvf $RUNID.tgz
SIZEOUT=$(cat ./"$RUNID"/single/0/run.out | wc -c)
echo "run.out is $SIZEOUT bytes."
SIZEERR=$(cat ./"$RUNID"/single/0/run.err | wc -c)
test $SIZEOUT -gt 0 && test $SIZEERR -eq 0
pushd $RUNID
OUTCOMEOK=$(find . | grep run.out | xargs awk '{print $0, FILENAME}' | grep "outcome\":\"ok\"" | wc -l)
test $OUTCOMEOK -eq 2
popd
popd
