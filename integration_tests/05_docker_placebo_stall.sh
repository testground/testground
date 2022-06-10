#!/bin/bash
my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground build single \
    --plan testground/placebo \
    --builder docker:go \
    --wait | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:placebo

testground healthcheck --runner local:docker --fix

testground run single \
    --plan=testground/placebo \
    --testcase=stall \
    --builder=docker:go \
    --use-build=testplan:placebo \
    --runner=local:docker \
    --instances=2 \
    --collect \
    --wait&

sleep 20
BEFORE=$(docker ps | grep placebo | wc -l)
testground terminate --runner=local:docker
sleep 10 
AFTER=$(docker ps | grep placebo | wc -l)
test $BEFORE -gt $AFTER

popd

echo "terminating remaining containers"
testground terminate --runner local:docker
testground terminate --builder docker:go
