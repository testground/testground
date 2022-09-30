#!/bin/bash
# Test for https://github.com/testground/testground/issues/1357
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans/_integrations_mixed_builders --name integrations_mixed_builders

pushd $TEMPDIR

testground healthcheck --runner local:docker --fix

testground build composition -f ${my_dir}/../plans/_integrations_mixed_builders/_compositions/issue-1357-mix-builder-configuration.toml \
    --wait | tee run.out

testground run composition -f ${my_dir}/../plans/_integrations_mixed_builders/_compositions/issue-1357-mix-builder-configuration.toml \
    --collect \
    --wait | tee run.out

SKIP_LOG_PARSING=true assert_run_output_is_correct run.out

popd

echo "terminating remaining containers"
testground terminate --runner local:docker
testground terminate --builder docker:go
# testground terminate --builder docker:generic # "component docker:generic is not terminatable" for now
