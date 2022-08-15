#!/bin/bash
# Test for https://github.com/testground/testground/issues/1357
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans/_integrations_mixed_builders --name integrations_mixed_builders

pushd $TEMPDIR

testground build composition -f ${my_dir}/../plans/_integrations_mixed_builders/_compositions/issue-1412-path-and-go-dependencies.toml \
   --wait

popd