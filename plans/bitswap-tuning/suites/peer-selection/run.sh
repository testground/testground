#!/bin/bash -x

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

OUTPUT_DIR=$1

if [ -z "$OUTPUT_DIR" ]; then
	TIMESTAMP=$(date +%Y-%m-%d-%T)
	OUTPUT_DIR="/tmp/bitswap-tuning-output/${TIMESTAMP}"
fi

mkdir -p $OUTPUT_DIR

runTest () {
	REF=$1
	REF_NAME=$2
	LABEL=$3
	RUN_COUNT=$4
	SEEDS=$5
	LEECHES=$6
	((INSTANCES=$SEEDS+$LEECHES))
	LATENCY_MS=$7
	BANDWIDTH_MB=$8
	FILESIZE=$9
	SEED_LATENCY=${10}
	TIMEOUT_SECS=${11}

	BRANCH_DIR="${OUTPUT_DIR}/${REF_NAME}"
	mkdir -p $BRANCH_DIR
	OUTFILE_BASE="${BRANCH_DIR}/${SEEDS}sx${LEECHES}l-${LATENCY_MS}ms-bw${BANDWIDTH_MB}.${LABEL}"
	OUTFILE_RAW="${OUTFILE_BASE}.raw"
	OUTFILE_CSV_BASE="${BRANCH_DIR}/${SEEDS}sx${LEECHES}l"
	./testground run bitswap-tuning/transfer \
	  --builder=docker:go \
	  --runner=local:docker \
	  --build-cfg bypass_cache=true \
	  --dep="github.com/ipfs/go-bitswap=$REF" \
	  --instances=$INSTANCES \
	  --test-param timeout_secs=$TIMEOUT_SECS \
	  --test-param run_count=$RUN_COUNT \
	  --test-param leech_count=$LEECHES \
	  --test-param latency_ms=$LATENCY_MS \
	  --test-param bandwidth_mb=$BANDWIDTH_MB \
	  --test-param file_size=$FILESIZE \
	  --test-param seed_latency_ms=$SEED_LATENCY \
	  --run-cfg log_file=$OUTFILE_RAW
	cat $OUTFILE_RAW | node $SCRIPT_DIR/peer-choice-process.js $OUTFILE_CSV_BASE
}

BW=64 # bandwidth in MB
LTCY=100
SIZES=1048576,2097152,4194304,8388608,16777216,33554432,67108864
SEED_LTCY=100,500,1000
TIMEOUT=1200
ITERATIONS=5
LABEL='1-64MB'

# 4 seed / 1 leech
runTest 'master' 'master' $LABEL $ITERATIONS 4 1 $LTCY $BW $SIZES $SEED_LTCY $TIMEOUT

# 4 seed / 1 leech
runTest '65321e4' 'poc' $LABEL $ITERATIONS 4 1 $LTCY $BW $SIZES $SEED_LTCY $TIMEOUT

echo "Output: $OUTPUT_DIR"
