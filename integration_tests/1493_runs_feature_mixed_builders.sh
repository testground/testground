#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

# run all instances
testground run composition \
    -f ${my_dir}/../plans/_integrations_runs/_compositions/issue-1493-happy-mix-builders-and-groups.toml \
    --collect                     \
    --result-file=./results.csv \
    --wait | tee run.out

assert_runs_outcome_are ./run.out success success success success
assert_runs_results ./results.csv success success success success

popd

echo "terminating remaining containers"
testground terminate --runner local:docker