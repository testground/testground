#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground run composition \
    -f ${my_dir}/../plans/placebo/_compositions/pr-1469-override-dependencies.toml \
    --collect \
    --wait | tee run.out

SKIP_LOG_PARSING=true assert_run_output_is_correct run.out

popd

echo "terminating remaining containers"
testground terminate --runner local:exec