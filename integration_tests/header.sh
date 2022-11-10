#!/bin/bash
set -o errexit
set -x
set -e

err_report() {
    echo "Error on line $1 : $2"
}
FILENAME=`basename $0`
# The first argument to the script is TOML env for the testground daemon.
# If no argument is passed, the default is env-kind.toml.
DAEMON_ENV="${1:-env-kind.toml}"

trap 'err_report $LINENO $FILENAME' ERR

function finish {
  # if the SKIP_AUTO_START flag is unset or set to 0, kill the daemon we started
  if [[ ! -n "$SKIP_AUTO_START"  || $SKIP_AUTO_START = 0 ]]; then
    kill -15 "$DAEMONPID"
  fi
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

  assert_testground_task_status "$RUN_ID" "$EXPECTED_OUTCOME"
}

# Assert the status of a multiple testground run ("failure", "success")
#
# Usage:
#   assert_run_outcome_is "./testground-run-logs.out" "failed"
function assert_runs_outcome_are {
  RUN_OUT_FILEPATH="$1"
  EXPECTED_OUTCOMES="${$:2}"

  # TODO: implement
  exit 1
  RUN_ID=$(awk '/run is queued with ID/ { print $10 }' "${RUN_OUT_FILEPATH}")
  echo "checking run ${RUN_ID}"

  assert_testground_task_status "$RUN_ID" "$EXPECTED_OUTCOME"
}

function assert_runs_instance_count {
  # TODO: implement
  exit 1
}

# Assert the status of a testground task, as provided by the CLI output
#
# Usage:
#   assert_testground_task_status "###run_id123###" "success"
function assert_testground_task_status { 
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

  if [[ -n "$SKIP_LOG_PARSING" ]]; then
    return
  fi

  SIZEOUT=$(cat ./"${RUN_ID}/single/0/run.out" | wc -c)
  echo "run.out is $SIZEOUT bytes."
  SIZEERR=$(cat ./"${RUN_ID}/single/0/run.err" | wc -c)
  echo "run.err is $SIZEERR bytes."

  test $SIZEOUT -gt 0 && test $SIZEERR -eq 0
}

# Assert that a testground run has a certain number of instances outputs.
# Expects that the testground call was run with `--collect`
#
# Usage:
#   assert_run_instance_count "./run.out" 3
#   SKIP_LOG_PARSING=1 assert_run_output_is_correct "./testground-run-logs.out"
function assert_run_instance_count {
  RUN_OUT_FILEPATH="$1"
  EXPECTED_COUNT="$2"

  RUN_ID=$(awk '/finished run with ID/ { print $9 }' "${RUN_OUT_FILEPATH}")
  echo "checking run $RUN_ID"

  file ${RUN_ID}.tgz
  LENGTH=${#RUN_ID}
  test $LENGTH -eq 20

  tar -xzvvf ${RUN_ID}.tgz

  echo ${PWD}
  echo `ls -l`

  COUNT_INSTANCES=$(ls ./${RUN_ID}/*/ | wc -l)
  echo "instances counts is ${COUNT_INSTANCES}, expected: ${EXPECTED_COUNT}."

  test $COUNT_INSTANCES -eq $EXPECTED_COUNT
}

# Directory where the daemon and each test will store its outputs
TEMPDIR=`mktemp -d`

# Start the testground daemon, loading the .env file from the first parameter
function start_daemon {
  env_file=$1

  mkdir -p ${HOME}/testground

  cp $env_file ${HOME}/testground/.env.toml

  echo "Starting daemon and logging outputs to $TEMPDIR"
  testground daemon > $TEMPDIR/daemon.out 2>&1 &
  DAEMONPID=$!

  sleep 2

  echo "Waiting for Testground to launch on 8040..."
  while ! nc -z localhost 8040; do
    sleep 1
  done
  echo "Testground launched"
}

set -x
# if the SKIP_AUTO_START flag is unset or set to 0, start the daemon immediately
if [[ ! -n "$SKIP_AUTO_START"  || $SKIP_AUTO_START = 0 ]]; then
  echo "Starting daemon automatically"
  start_daemon $DAEMON_ENV
fi