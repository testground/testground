#!/bin/bash
# Test for https://github.com/testground/testground/issues/1236
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Use our preset env file, where the local:docker runner is blocked
source "$my_dir/header.sh" "$my_dir/18_env.toml"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground healthcheck --runner local:docker --fix

# Run test
testground run single \
    --plan=testground/placebo \
    --testcase=ok \
    --builder=docker:go \
    --runner=local:docker \
    --instances=2 \
    --wait | tee run.out

assert_run_outcome_is ./run.out "canceled"

popd