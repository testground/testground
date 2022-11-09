#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

# testground run composition \
#     -f ${my_dir}/../plans/_integrations_runs/_compositions/issue-1493-multiple-runs-happy-path.toml \
#     --collect \
#     --wait | tee run.out

# assert_run_outcome_is ./run.out "failure"

testground run composition \
    -f ${my_dir}/../plans/_integrations_runs/_compositions/issue-1493-multiple-runs-happy-path.toml \
    --run-ids="run_simple_4" \
    --collect   \
    --wait | tee run.out

assert_run_outcome_is ./run.out "success"
assert_run_instance_count ./run.out 4

popd

echo "terminating remaining containers"
testground terminate --runner local:docker