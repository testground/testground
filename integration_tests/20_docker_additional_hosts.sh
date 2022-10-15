#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh" "$my_dir/20_env.toml"

testground healthcheck --runner local:docker --fix

# From https://hub.docker.com/r/hashicorp/http-echo/
docker run -d -p 5678:5678 --name http-echo hashicorp/http-echo -text="ok"
docker network connect testground-control http-echo

testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground build single \
    --plan testground/additional_hosts \
    --builder docker:go \
    --wait | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:additional_hosts


testground run single \
    --plan=testground/additional_hosts \
    --testcase=additional_hosts \
    --builder=docker:go \
    --use-build=testplan:additional_hosts \
    --runner=local:docker \
    --instances=1 \
    --collect \
    --wait | tee run.out

assert_run_output_is_correct run.out
assert_run_outcome_is ./run.out "success"

popd

echo "terminating remaining containers"
docker rm -f http-echo
testground terminate --runner local:docker
testground terminate --builder docker:go
