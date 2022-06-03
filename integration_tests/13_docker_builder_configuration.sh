#!/bin/bash

# Test for https://github.com/testground/testground/issues/1337

__dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

source "${__dir}/header.sh"

testground plan import --from ./plans/_integrations --name integrations

pushd $TEMPDIR

testground healthcheck --runner local:docker --fix
testground run composition -f ${__dir}/../plans/_integrations/_compositions/issue-1337-groups-builder-configuration.toml --collect --wait | tee stdout.out

RUNID=$(awk '/finished run with ID/ { print $9 }' stdout.out)
echo "checking run $RUNID"
file $RUNID.tgz
tar -xzvvf $RUNID.tgz
SIZEOUT=$(cat ./"$RUNID"/*/0/run.out | wc -c)
echo "run.out is $SIZEOUT bytes."
SIZEERR=$(cat ./"$RUNID"/*/0/run.err | wc -c)
test $SIZEOUT -gt 0 && test $SIZEERR -eq 0
popd

echo "terminating remaining containers"
testground terminate --runner local:docker
testground terminate --builder docker:go
