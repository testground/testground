#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground build single \
    --plan testground/placebo \
    --builder docker:go \
    --wait | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:placebo

# The placebo:stall does not require a sidecar.
# To prevent kind from attempting to download the image from DockerHub, build and load the image before executing it.
# The plan is renamed as `testplan:placebo` because kind will check DockerHub if the tag is `latest`.
kind load docker-image testplan:placebo

testground healthcheck --runner local:docker --fix

testground run single \
    --plan=testground/placebo \
    --testcase=stall \
    --builder=docker:go \
    --use-build=testplan:placebo \
    --runner=cluster:k8s \
    --instances=2 \
    --collect \
    --wait&

sleep 20
BEFORE=$(kubectl get pods | grep placebo | grep Running | wc -l)
testground terminate --runner=cluster:k8s
sleep 10
AFTER=$(kubectl get pods | grep placebo | grep Running | wc -l)
test $BEFORE -gt $AFTER

popd

echo "terminating remaining containers"
testground terminate --builder docker:go
