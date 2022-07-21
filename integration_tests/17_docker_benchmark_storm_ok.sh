#!/bin/bash
# Test for https://github.com/testground/testground/pull/1393
# (Benchmark storm test)
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground healthcheck --runner local:docker --fix

# Run test
testground run single \
    --plan=testground/benchmarks \
    --testcase=storm \
    --builder=docker:go \
    --runner=local:docker \
    --instances=2 \
    --wait | tee run.out

assert_run_outcome_is ./run.out "success"

popd