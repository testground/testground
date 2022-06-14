#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground healthcheck --runner local:docker --fix

testground run single \
    --plan=testground/example-rust \
    --testcase=tcp-connect \
    --builder=docker:generic \
    --runner=local:docker \
    --instances=2 \
    --collect \
    --wait | tee run.out

SKIP_LOG_PARSING=true assert_run_output_is_correct run.out

popd

echo "terminating remaining containers"
testground terminate --runner local:docker