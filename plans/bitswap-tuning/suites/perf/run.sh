#!/bin/bash -x

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

OUTPUT_DIR=$1
if [ -z "$OUTPUT_DIR" ]; then
	TIMESTAMP=$(date +%Y-%m-%d-%T)
	OUTPUT_DIR="/tmp/bitswap-tuning-output/${TIMESTAMP}"
fi

BW=$2 # bandwidth in MB
if [ -z "$BW" ]; then
	BW=1024
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
	TIMEOUT_SECS=${10}

	BRANCH_DIR="${OUTPUT_DIR}/${REF_NAME}"
	mkdir -p $BRANCH_DIR
	OUTFILE_BASE="${BRANCH_DIR}/${SEEDS}sx${LEECHES}l-${LATENCY_MS}ms-bw${BANDWIDTH_MB}.${LABEL}"
	OUTFILE_RAW="${OUTFILE_BASE}.raw"
	OUTFILE_CSV_BASE="${BRANCH_DIR}/${SEEDS}sx${LEECHES}l"
	./testground run single bitswap-tuning/transfer \
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
	  --run-cfg log_file=$OUTFILE_RAW
	RUN_ID=`ls -lt ~/.testground/local_docker/outputs/bitswap-tuning | head -2 | tail -1 | awk '{print $NF}'`
	OUTZIP="${OUTPUT_DIR}/${RUN_ID}.zip"
	./testground collect --runner=local:docker --output=$OUTZIP $RUN_ID
	unzip $OUTZIP -d $OUTPUT_DIR
	cat ${OUTPUT_DIR}/${RUN_ID}/single/*/run.out | node $SCRIPT_DIR/aggregate.js $OUTFILE_CSV_BASE
	node $SCRIPT_DIR/chart.js -d $OUTPUT_DIR -m time_to_fetch -b $BANDWIDTH_MB -l $LATENCY_MS -xlabel 'File size (MB)' -ylabel 'Time to fetch (s)' -xscale '9.53674316e-7' -yscale '1e-9'
	gnuplot $OUTPUT_DIR/time_to_fetch.plot > $OUTPUT_DIR/time_to_fetch.svg
}

LTCY=100
SIZES=1048576,2097152,4194304,8388608,16777216,33554432,47453132,56431603,67108864
TIMEOUT=1200
ITERATIONS=5
LABEL='1-64MB'

# 1 seed / 1 leech
runTest 'dcfe40e' 'old' $LABEL $ITERATIONS 1 1 $LTCY $BW $SIZES $TIMEOUT
# 2 seed / 1 leech
runTest 'dcfe40e' 'old' $LABEL $ITERATIONS 2 1 $LTCY $BW $SIZES $TIMEOUT
# 4 seed / 1 leech
runTest 'dcfe40e' 'old' $LABEL $ITERATIONS 4 1 $LTCY $BW $SIZES $TIMEOUT

# 1 seed / 1 leech
runTest 'master' 'new' $LABEL $ITERATIONS 1 1 $LTCY $BW $SIZES $TIMEOUT
# 2 seed / 1 leech
runTest 'master' 'new' $LABEL $ITERATIONS 2 1 $LTCY $BW $SIZES $TIMEOUT
# 4 seed / 1 leech
runTest 'master' 'new' $LABEL $ITERATIONS 4 1 $LTCY $BW $SIZES $TIMEOUT

echo "Output: $OUTPUT_DIR"
