#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground build single \
    --plan testground/just_sleeping \
    --builder docker:go \
    --wait | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:sleep

testground healthcheck --runner local:docker --fix

testground run single \
    --plan=testground/just_sleeping \
    --testcase=sleep \
    --builder=docker:go \
    --use-build=testplan:sleep \
    --runner=local:docker \
    --instances=1 \
    --wait | tee run.out

assert_run_outcome_is ./run.out "failure"

popd
