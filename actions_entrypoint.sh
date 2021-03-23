#!/usr/local/bin/env bash

# Positoinal arguments -- see action.yml
BACKEND=$1
PLANDIR=$2
COMPFILE=$3

# exit codes determine the result of the CheckRun
# https://docs.github.com/en/actions/creating-actions/setting-exit-codes-for-actions
SUCCESS=0
FAILURE=1

echo hello, action
echo $BACKEND $PLANDIR $COMPFILE

exit $SUCCESS
