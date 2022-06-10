#!/bin/bash
my_dir="$(dirname "$0")"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground build single \
    --plan testground/placebo \
    --builder docker:go \
    --wait | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:placebo

# The placebo:ok does not require a sidecar.
# To prevent kind from attempting to download the image from DockerHub, build and load the image before executing it.
# The plan is renamed as `testplan:placebo` because kind will check DockerHub if the tag is `latest`.
kind load docker-image testplan:placebo

testground healthcheck --runner local:docker --fix

testground run single \
    --plan=testground/placebo \
    --testcase=ok \
    --builder=docker:go \
    --use-build=testplan:placebo \
    --runner=cluster:k8s \
    --instances=1 \
    --collect \
    --wait | tee run.out

assert_run_output_is_correct run.out

popd

echo "terminating remaining containers"
testground terminate --builder docker:go
