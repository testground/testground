#!/bin/bash

my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

testground plan import --from plans/placebo
testground build single --builder docker:go --plan placebo | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:placebo

testground healthcheck --runner local:docker --fix
testground run single --runner local:docker --builder docker:go --use-build testplan:placebo --instances 2 --plan placebo --testcase stall --wait &
sleep 20
BEFORE=$(docker ps | grep placebo | wc -l)
testground terminate --runner=local:docker
sleep 10 
AFTER=$(docker ps | grep placebo | wc -l)
test $BEFORE -gt $AFTER

echo "terminating remaining containers"
testground terminate --runner local:docker
testground terminate --builder docker:go
