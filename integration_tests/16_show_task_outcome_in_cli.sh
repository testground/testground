#!/bin/bash
# Test for https://github.com/testground/testground/issues/1346
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground healthcheck --runner local:docker --fix

# Test with success
testground run single \
    --plan=testground/placebo \
    --testcase=ok \
    --builder=docker:go \
    --runner=local:docker \
    --instances=1 \
    --wait | tee run.out

assert_run_outcome_is ./run.out "success"

# Test with failure
testground run single \
    --plan=testground/placebo \
    --testcase=panic \
    --builder=docker:go \
    --runner=local:docker \
    --instances=1 \
    --wait | tee run.out

assert_run_outcome_is ./run.out "failure"

popd