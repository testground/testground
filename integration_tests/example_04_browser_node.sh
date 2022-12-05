#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

docker pull node:16-buster
testground plan import --from ./plans --name testground

pushd $TEMPDIR

testground build single \
    --plan testground/example-browser-node \
    --builder docker:generic \
    --wait | tee build.out
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)
docker tag $ARTIFACT testplan:example-browser-node

testground healthcheck --runner local:docker --fix

# Node: success

testground run single \
    --plan=testground/example-browser-node \
    --testcase=output \
    --builder=docker:generic \
    --use-build=testplan:example-browser-node \
    --runner=local:docker \
    --instances=1 \
    --collect \
    --wait | tee run.out

assert_run_outcome_is run.out "success"

# Browser (Chromium): success

testground run single \
    --plan=testground/example-browser-node \
    --testcase=output \
    --builder=docker:generic \
    --use-build=testplan:example-browser-node \
    --runner=local:docker \
    --instances=1 \
    --tp runtime=chromium \
    --collect \
    --wait | tee run.out

assert_run_outcome_is run.out "success"

# Browser (Firefox): success

testground run single \
    --plan=testground/example-browser-node \
    --testcase=output \
    --builder=docker:generic \
    --use-build=testplan:example-browser-node \
    --runner=local:docker \
    --instances=1 \
    --tp runtime=firefox \
    --collect \
    --wait | tee run.out

assert_run_outcome_is run.out "success"

# Browser (WebKit): success

testground run single \
    --plan=testground/example-browser-node \
    --testcase=output \
    --builder=docker:generic \
    --use-build=testplan:example-browser-node \
    --runner=local:docker \
    --instances=1 \
    --tp runtime=webkit \
    --collect \
    --wait | tee run.out

assert_run_outcome_is run.out "success"

# Node: failure

testground run single \
    --plan=testground/example-browser-node \
    --testcase=failure \
    --builder=docker:generic \
    --use-build=testplan:example-browser-node \
    --runner=local:docker \
    --instances=1 \
    --collect \
    --wait | tee run.out

assert_run_outcome_is run.out "failure"

# Browser (Chromium): failure

testground run single \
    --plan=testground/example-browser-node \
    --testcase=failure \
    --builder=docker:generic \
    --use-build=testplan:example-browser-node \
    --runner=local:docker \
    --instances=1 \
    --tp runtime=chromium \
    --collect \
    --wait | tee run.out

assert_run_outcome_is run.out "failure"

# Browser (Firefox): failure

testground run single \
    --plan=testground/example-browser-node \
    --testcase=failure \
    --builder=docker:generic \
    --use-build=testplan:example-browser-node \
    --runner=local:docker \
    --instances=1 \
    --tp runtime=firefox \
    --collect \
    --wait | tee run.out

assert_run_outcome_is run.out "failure"

# Browser (WebKit): failure

testground run single \
    --plan=testground/example-browser-node \
    --testcase=failure \
    --builder=docker:generic \
    --use-build=testplan:example-browser-node \
    --runner=local:docker \
    --instances=1 \
    --tp runtime=webkit \
    --collect \
    --wait | tee run.out

assert_run_outcome_is run.out "failure"

# Node: sync

testground run single \
    --plan=testground/example-browser-node \
    --testcase=sync \
    --builder=docker:generic \
    --use-build=testplan:example-browser-node \
    --runner=local:docker \
    --instances=2 \
    --collect \
    --wait | tee run.out

assert_run_outcome_is run.out "success"

# Browser (Chromium): sync

testground run single \
    --plan=testground/example-browser-node \
    --testcase=sync \
    --builder=docker:generic \
    --use-build=testplan:example-browser-node \
    --runner=local:docker \
    --instances=2 \
    --tp runtime=chromium \
    --collect \
    --wait | tee run.out

assert_run_outcome_is run.out "success"

# Browser (Firefox): sync

testground run single \
    --plan=testground/example-browser-node \
    --testcase=sync \
    --builder=docker:generic \
    --use-build=testplan:example-browser-node \
    --runner=local:docker \
    --instances=2 \
    --tp runtime=firefox \
    --collect \
    --wait | tee run.out

assert_run_outcome_is run.out "success"

# Browser (WebKit): sync

testground run single \
    --plan=testground/example-browser-node \
    --testcase=sync \
    --builder=docker:generic \
    --use-build=testplan:example-browser-node \
    --runner=local:docker \
    --instances=2 \
    --tp runtime=webkit \
    --collect \
    --wait | tee run.out

assert_run_outcome_is run.out "success"

popd

echo "terminating remaining containers"
testground terminate --runner local:docker