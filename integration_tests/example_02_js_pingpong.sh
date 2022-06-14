#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

docker pull node:16-buster
testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground build single \
    --plan testground/example-js \
    --builder docker:node \
    --wait | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:example-js

testground healthcheck --runner local:docker --fix

testground run single \
    --plan=testground/example-js \
    --testcase=pingpong \
    --builder=docker:node \
    --use-build=testplan:example-js \
    --runner=local:docker \
    --instances=2 \
    --collect \
    --wait | tee run.out

assert_run_output_is_correct run.out

popd

echo "terminating remaining containers"
testground terminate --runner local:docker