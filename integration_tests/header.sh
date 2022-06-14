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
