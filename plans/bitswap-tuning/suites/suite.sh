#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
TIMESTAMP=$(date +%Y-%m-%d-%T)
OUTPUT_DIR="/tmp/bitswap-tuning-output/${TIMESTAMP}"
mkdir -p $OUTPUT_DIR

runTest () {
	SEEDS=$1
	LEECHES=$2
	((INSTANCES=$SEEDS+$LEECHES))
	OUTFILE_RAW="${OUTPUT_DIR}/${SEEDS}sx${LEECHES}l.raw"
	OUTFILE="${OUTPUT_DIR}/${SEEDS}sx${LEECHES}l.txt"
	./testground run bitswap-tuning/transfer \
	  --builder=docker:go \
	  --runner=local:docker \
	  --build-cfg bypass_cache=true \
	  --instances=$INSTANCES \
	  --test-param leech_count=$LEECHES \
	  --run-cfg log_file=$OUTFILE_RAW
	node $SCRIPT_DIR/aggregate.js $OUTFILE_RAW | node $SCRIPT_DIR/table.js > $OUTFILE
}

runTest 1 1
runTest 2 1
runTest 1 2
runTest 1 4
echo "Output: $OUTPUT_DIR"
