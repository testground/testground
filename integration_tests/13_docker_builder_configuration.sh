#!/bin/bash
# Test for https://github.com/testground/testground/issues/1337
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans/_integrations_interop --name integrations_interop

pushd $TEMPDIR

testground healthcheck --runner local:docker --fix

testground run composition -f ${my_dir}/../plans/_integrations_interop/_compositions/issue-1337-groups-builder-configuration.toml \
    --collect \
    --wait | tee run.out

SKIP_LOG_PARSING=true assert_run_output_is_correct run.out

popd

echo "terminating remaining containers"
testground terminate --runner local:docker
testground terminate --builder docker:go
