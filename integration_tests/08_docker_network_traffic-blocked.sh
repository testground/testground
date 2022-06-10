#!/bin/bash
my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground build single \
    --plan testground/network \
    --builder docker:go \
    --wait | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:network

testground healthcheck --runner local:docker --fix

testground run single \
    --plan=testground/network \
    --testcase=traffic-blocked \
    --builder=docker:go \
    --use-build=testplan:network \
    --runner=local:docker \
    --instances=1 \
    --collect \
    --wait | tee run.out

assert_run_output_is_correct run.out

popd

echo "terminating remaining containers"
testground terminate --runner local:docker
testground terminate --builder docker:go
