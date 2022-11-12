#!/bin/sh

if [ -z "$TESTGROUND_HELLO_MESSAGE" ]; then
    echo "Need to set TESTGROUND_HELLO_MESSAGE"
    exit 1
elif [ "$TESTGROUND_HELLO_MESSAGE" != "Hello, AdditionalEnvs!" ]; then
    echo "TESTGROUND_HELLO_MESSAGE found with incorrect value: ${TESTGROUND_HELLO_MESSAGE}"
    exit 1
else
    echo "TESTGROUND_HELLO_MESSAGE found and correct"
    exit 0
fi
