#!/bin/bash

# Test for https://github.com/testground/testground/issues/TBD

__dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source "${__dir}/header.sh"

testground plan import --from ./plans/_integrations --name integrations

pushd $TEMPDIR

testground healthcheck --runner local:docker --fix

testground run single --plan=integrations --testcase="issue-1349-silent-failure" \
    --builder=docker:go --runner=local:docker \
    --instances=1 --wait;

RESULT=$?

echo "terminating remaining containers"
testground terminate --runner local:docker
testground terminate --builder docker:go

if [ RESULT = 0 ]; then
    echo "Test succeeded, but should have failed."
    exit 1
fi