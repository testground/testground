#!/bin/bash
set -o errexit
set -x
set -e

err_report() {
    echo "Error on line $1 : $2"
}
FILENAME=`basename $0`
trap 'err_report $LINENO $FILENAME' ERR

function finish {
  kill -15 $DAEMONPID
}
trap finish EXIT

# Assert the status of a testground run ("failure", "success")
#
# Usage:
#   assert_run_outcome_is "./testground-run-logs.out" "failed"
function assert_run_outcome_is {
  RUN_OUT_FILEPATH="$1"
  EXPECTED_OUTCOME="$2"

  RUN_ID=$(awk '/run is queued with ID/ { print $10 }' "${RUN_OUT_FILEPATH}")
  echo "checking run ${RUN_ID}"

  check_testground_task_status "$RUN_ID" "$EXPECTED_OUTCOME"
}

# Assert the status of a testground task, as provided by the CLI output
#
# Usage:
#   check_testground_task_status "###run_id123###" "success"
function check_testground_task_status { 
  RUN_ID="$1"
  EXPECTED_OUTCOME="$2"

  OUTCOME=$(testground status -t "${RUN_ID}" | awk '/Outcome:/{ print $2 }')

  if [ "${OUTCOME}" != "${EXPECTED_OUTCOME}" ]; then
      exit 1;
  fi
}

# Assert that a testground run has no errors and some logs.
# use `SKIP_LOG_PARSING` to check for errors only (required when working with SDKs that don't output logs).
#
# Usage:
#   assert_run_output_is_correct "./testground-run-logs.out"
#   SKIP_LOG_PARSING=1 assert_run_output_is_correct "./testground-run-logs.out"
function assert_run_output_is_correct {
  RUN_OUT_FILEPATH="$1"

  RUN_ID=$(awk '/finished run with ID/ { print $9 }' "${RUN_OUT_FILEPATH}")
  echo "checking run $RUN_ID"

  file ${RUN_ID}.tgz
  LENGTH=${#RUN_ID}
  test $LENGTH -eq 20

  tar -xzvvf ${RUN_ID}.tgz

  if [ SKIP_LOG_PARSING ]; then
    return
  fi

  SIZEOUT=$(cat ./"${RUN_ID}/single/0/run.out" | wc -c)
  echo "run.out is $SIZEOUT bytes."
  SIZEERR=$(cat ./"${RUN_ID}/single/0/run.err" | wc -c)
  echo "run.err is $SIZEERR bytes."

  test $SIZEOUT -gt 0 && test $SIZEERR -eq 0
}

TEMPDIR=`mktemp -d`
mkdir -p ${HOME}/testground
cp env-kind.toml ${HOME}/testground/.env.toml

echo "Starting daemon and logging outputs to $TEMPDIR"
testground daemon > $TEMPDIR/daemon.out 2>&1 &
DAEMONPID=$!

sleep 2

echo "Waiting for Testground to launch on 8040..."
while ! nc -z localhost 8040; do
  sleep 1
done
echo "Testground launched"
