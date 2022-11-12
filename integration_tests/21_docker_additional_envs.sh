#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground healthcheck --runner local:docker --fix

testground plan import --from ./plans --name testground

pushd $TEMPDIR

# Generic Container

testground build single \
    --plan testground/additional_envs \
    --builder docker:generic \
    --wait | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:additional_envs


testground run single \
    --plan=testground/additional_envs \
    --testcase=additional_envs \
    --builder=docker:generic \
    --use-build=testplan:additional_envs \
    --runner=local:docker \
    --instances=1 \
    --collect \
    --wait | tee run.out

assert_run_output_is_correct run.out
assert_run_outcome_is ./run.out "success"

# Go Docker

testground build single \
    --plan testground/additional_envs \
    --builder docker:go \
    --wait | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:additional_envs


testground run single \
    --plan=testground/additional_envs \
    --testcase=additional_envs \
    --builder=docker:go \
    --use-build=testplan:additional_envs \
    --runner=local:docker \
    --instances=1 \
    --collect \
    --wait | tee run.out

assert_run_output_is_correct run.out
assert_run_outcome_is ./run.out "success"

popd

echo "terminating remaining containers"
docker rm -f http-echo
testground terminate --runner local:docker
testground terminate --builder docker:go
