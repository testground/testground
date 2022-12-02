#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

# run a single instance
testground run composition \
    -f ${my_dir}/../plans/_integrations_runs/_compositions/issue-1493-multiple-runs-happy-path.toml \
    --run-ids="run_simple_4" \
    --collect   \
    --wait | tee run.out

assert_run_outcome_is ./run.out "success"
assert_run_instance_count ./run.out 4

# run all instances
testground run composition \
    -f ${my_dir}/../plans/_integrations_runs/_compositions/issue-1493-multiple-runs-happy-path.toml \
    --collect                     \
    --result-file=./results.csv \
    --wait | tee run.out

assert_runs_outcome_are ./run.out "success" "success" "success"
# assert_runs_instance_count ./run.out 1 2 4 # TODO: IMPLEMENT
assert_runs_results ./results.csv success success success

popd

echo "terminating remaining containers"
testground terminate --runner local:docker