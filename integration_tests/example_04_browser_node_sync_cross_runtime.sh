#!/bin/bash
# Test for https://github.com/testground/testground/issues/1337
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans/example-browser-node --name example-browser-node

pushd $TEMPDIR

testground healthcheck --runner local:docker --fix

testground run composition -f ${my_dir}/../plans/example-browser-node/compositions/sync-cross-runtime.toml \
    --collect \
    --wait | tee run.out

assert_run_outcome_is run.out "success"

popd

echo "terminating remaining containers"
testground terminate --runner local:docker
testground terminate --builder docker:go
