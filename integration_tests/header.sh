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

TEMPDIR=`mktemp -d`
mkdir -p ${HOME}/testground
cp env-kind.toml ${HOME}/testground/.env.toml
echo Starting daemon and logging outputs to $TEMPDIR
testground daemon > $TEMPDIR/daemon.out 2>&1 &
DAEMONPID=$!

sleep 2

echo "Waiting for Testground to launch on 8080..."
while ! nc -z localhost 8080; do
  sleep 1
done
echo "Testground launched"
