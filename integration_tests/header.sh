#!/bin/bash

set -o errexit
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
testground daemon > daemon.out 2>&1 &
DAEMONPID=$!

