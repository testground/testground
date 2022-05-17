#!/bin/bash

# Test for https://github.com/testground/testground/issues/1337

my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

testground plan import --from ./plans/integrations

testground healthcheck --runner local:docker --fix
testground run composition -f ./plans/integrations/_compositions/issue-1337-override-builder-configuration.toml --collect --wait  | tee stdout.out

RUNID=$(awk '/finished run with ID/ { print $9 }' stdout.out)
echo "checking run $RUNID"
file $RUNID.tgz
LENGTH=${#RUNID}
test $LENGTH -eq 20
tar -xzvvf $RUNID.tgz
SIZEOUT=$(cat ./"$RUNID"/single/0/run.out | wc -c)
echo "run.out is $SIZEOUT bytes."
SIZEERR=$(cat ./"$RUNID"/single/0/run.err | wc -c)
test $SIZEOUT -gt 0 && test $SIZEERR -eq 0
popd

echo "terminating remaining containers"
testground terminate --runner local:docker
testground terminate --builder docker:go
