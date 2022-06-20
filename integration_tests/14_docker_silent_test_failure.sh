#!/bin/bash
# Test for https://github.com/testground/testground/issues/1349
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans/_integrations --name integrations

pushd $TEMPDIR

testground healthcheck --runner local:docker --fix

# Note the `&& exit 1` lets us fail the test if the run succeeds.
# We can't rely on $? when the run fails, because the `err_report` function in header.sh
# runs first and succeeds (so $? is always 0).
testground run single --plan=integrations --testcase="issue-1349-silent-failure" \
    --builder=docker:go --runner=local:docker \
    --instances=1 --wait && exit 1;

popd

echo "terminating remaining containers"
testground terminate --runner local:docker
testground terminate --builder docker:go
