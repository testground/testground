#!/bin/bash
# Test for https://github.com/testground/testground/issues/1236
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# block the daemon from starting automatically
AUTO_START=1
source "$my_dir/header.sh"

# start it manually with the preset env file, where the local:docker runner is blocked
start_daemon "$my_dir/18_env.toml"

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