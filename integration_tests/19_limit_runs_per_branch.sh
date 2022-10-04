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

testground healthcheck --runner local:docker --fix

# Run first test case, but do not --wait
# We must start the 2nd case at the same time
t1=$(testground run single \
    --plan=testground/placebo \
    --testcase=ok \
    --builder=docker:go \
    --use-build=testplan:placebo \
    --runner=local:docker \
    --instances=1 \
    --metadata-branch b1 \
    --metadata-repo r1)

# Run the second test case, and wait for it to finish
t2=$(testground run single \
    --plan=testground/placebo \
    --testcase=ok \
    --builder=docker:go \
    --use-build=testplan:placebo \
    --runner=local:docker \
    --instances=1 \
    --metadata-branch b1 \
    --metadata-repo r1 \
    --wait \
    --collect)

# Very unwieldy, $59 is the ID in the log
run_id1=$(echo $t1 | awk '/run is queued with ID: / {print $59}')
run_id2=$(echo $t2 | awk '/run is queued with ID: / {print $59}')

# First run must be canceled
assert_testground_task_status "$run_id1" 'canceled'
# Second must succeed
assert_testground_task_status "$run_id2" 'success'

popd
