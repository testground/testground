#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

# run all instances
testground run composition \
    -f ${my_dir}/../plans/_integrations_runs/_compositions/issue-1493-break-the-build.toml \
    --collect                   \
    --result-file=./results.csv \
    --wait | tee run.out

assert_runs_outcome_are ./run.out canceled canceled canceled
assert_runs_results ./results.csv canceled canceled canceled

popd

echo "terminating remaining containers"
testground terminate --runner local:docker