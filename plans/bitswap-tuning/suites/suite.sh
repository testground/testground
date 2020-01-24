#!/bin/bash -x

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

OUTPUT_DIR=$1
if [ -z "$OUTPUT_DIR" ]; then
	TIMESTAMP=$(date +%Y-%m-%d-%T)
	OUTPUT_DIR="/tmp/bitswap-tuning-output/${TIMESTAMP}"
fi

mkdir -p $OUTPUT_DIR

runTest () {
	LABEL=$1
	RUN_COUNT=$2
	SEEDS=$3
	LEECHES=$4
	((INSTANCES=$SEEDS+$LEECHES))
	LATENCY_MS=$5
	BANDWIDTH_MB=$6
	FILESIZE=$7
	TIMEOUT_SECS=$8

	OUTFILE_BASE="${OUTPUT_DIR}/${SEEDS}sx${LEECHES}l-${LATENCY_MS}ms-bw${BANDWIDTH_MB}.${LABEL}"
	OUTFILE_RAW="${OUTFILE_BASE}.raw"
	OUTFILE_CSV_BASE="${OUTPUT_DIR}/${SEEDS}sx${LEECHES}l"
	./testground run bitswap-tuning/transfer \
	  --builder=docker:go \
	  --runner=local:docker \
	  --build-cfg bypass_cache=true \
	  --dep="github.com/ipfs/go-bitswap=65321e4" \
	  --instances=$INSTANCES \
	  --test-param timeout_secs=$TIMEOUT_SECS \
	  --test-param run_count=$RUN_COUNT \
	  --test-param leech_count=$LEECHES \
	  --test-param latency_ms=$LATENCY_MS \
	  --test-param bandwidth_mb=$BANDWIDTH_MB \
	  --test-param file_size=$FILESIZE \
	  --run-cfg log_file=$OUTFILE_RAW
	cat $OUTFILE_RAW | node $SCRIPT_DIR/aggregate.js $OUTFILE_CSV_BASE
}

# 1 seed / 1 leech
runTest '1-64MB' 5 1 1 5 1024 1048576,2097152,4194304,8388608,16777216,33554432,47453132,56431603,67108864 240
# 2 seed / 1 leech
runTest '1-64MB' 5 2 1 5 1024 1048576,2097152,4194304,8388608,16777216,33554432,47453132,56431603,67108864 240
# 4 seed / 1 leech
runTest '1-64MB' 5 4 1 5 1024 1048576,2097152,4194304,8388608,16777216,33554432,47453132,56431603,67108864 480

echo "Output: $OUTPUT_DIR"
