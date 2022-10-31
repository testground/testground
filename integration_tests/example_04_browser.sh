#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

docker pull node:16
docker pull mcr.microsoft.com/playwright:v1.25.2-focal
testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground build single \
    --plan testground/example-browser \
    --builder docker:generic \
    --wait | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:example-browser

testground healthcheck --runner local:docker --fix

# Test Chromium with success
testground run single \
    --plan=testground/example-browser \
    --testcase=success \
    --builder=docker:generic \
    --use-build=testplan:example-browser \
    --runner=local:docker \
    --instances=1 \
    --tp browser=chromium \
    --collect \
    --wait | tee run.out

assert_run_outcome_is ./run.out "success"

# Test Firefox with success
testground run single \
    --plan=testground/example-browser \
    --testcase=success \
    --builder=docker:generic \
    --use-build=testplan:example-browser \
    --runner=local:docker \
    --instances=1 \
    --tp browser=firefox \
    --collect \
    --wait | tee run.out

assert_run_outcome_is ./run.out "success"

# Test Chromium with failure
testground run single \
    --plan=testground/example-browser \
    --testcase=failure \
    --builder=docker:generic \
    --use-build=testplan:example-browser \
    --runner=local:docker \
    --instances=1 \
    --tp browser=chromium \
    --collect \
    --wait | tee run.out

assert_run_outcome_is ./run.out "failure"

# Test Firefox with failure
testground run single \
    --plan=testground/example-browser \
    --testcase=failure \
    --builder=docker:generic \
    --use-build=testplan:example-browser \
    --runner=local:docker \
    --instances=1 \
    --tp browser=firefox \
    --collect \
    --wait | tee run.out

assert_run_outcome_is ./run.out "failure"

popd

echo "terminating remaining containers"
testground terminate --runner local:docker