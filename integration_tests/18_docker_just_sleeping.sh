#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans/_integrations --name integrations

pushd $TEMPDIR

testground healthcheck --runner local:docker --fix

testground run single \
    --plan=integrations \
    --testcase="issue-1432-task-timeout" \
    --builder=docker:go \
    --runner=local:docker \
    --instances=1 \
    --wait | tee run.out

assert_run_outcome_is ./run.out "failure"

popd
