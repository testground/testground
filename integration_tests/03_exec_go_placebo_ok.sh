#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground run single \
    --plan=testground/placebo \
    --testcase=ok \
    --builder=exec:go \
    --runner=local:exec \
    --instances=2 \
    --collect \
    --wait | tee run.out

assert_run_output_is_correct run.out

popd

echo "terminating remaining containers"
testground terminate --runner local:exec