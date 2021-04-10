#!/bin/bash

my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

testground plan import --from plans/placebo
testground build single --builder docker:go --plan placebo --wait | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:placebo

# The placebo:ok does not require a sidecar.
# To prevent kind from attempting to download the image from DockerHub, build and load the image before executing it.
# The plan is renamed as `testplan:placebo` because kind will check DockerHub if the tag is `latest`.
kind load docker-image testplan:placebo

pushd $TEMPDIR
testground healthcheck --runner local:docker --fix
export SYNC_SERVICE_HOST=localhost
testground run single --runner cluster:k8s --builder docker:go --use-build testplan:placebo --instances 1 --plan placebo --testcase ok --collect --wait | tee run.out
RUNID=$(awk '/finished run with ID/ { print $9 }' run.out)
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
testground terminate --builder docker:go
