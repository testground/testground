#!/bin/bash -x

# run.sh /tmp/run1 one-leech-two-seeds/def.js

LATENCY_MS=100
BANDWIDTH_MB=1024

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

OUTPUT_DIR=$1
if [ -z "$OUTPUT_DIR" ]; then
	TIMESTAMP=$(date +%Y-%m-%d-%T)
	OUTPUT_DIR="/tmp/bitswap-tuning-output/${TIMESTAMP}"
fi

DEF_JS=$2

mkdir -p $OUTPUT_DIR

# Generate toml files
node ${SCRIPT_DIR}/gentomls.js -o $OUTPUT_DIR -def $DEF_JS \
  --latency_ms=$LATENCY_MS \
  --bandwidth_mb=$BANDWIDTH_MB \
  --file_size=1048576,2097152,4194304,8388608,16777216,33554432,67108864 \
  --run_count=5 \
  --seed_fraction=2/3

# Use each toml to run testground
RES_DIR="${OUTPUT_DIR}/results"
FILES=$OUTPUT_DIR/*.toml
for FILE in $FILES; do
	# First line is "# title"
	TITLE=$(head -1 $FILE)
	TITLE=${TITLE:2}

	LABEL=$TITLE
	LABEL_DIR="${RES_DIR}/${LABEL}"
	mkdir -p $LABEL_DIR
	OUTFILE_CSV_BASE="${LABEL_DIR}/out"

	./testground run c -f $FILE
	RUN_ID=`ls -lt ~/.testground/local_docker/outputs/bitswap-tuning | head -2 | tail -1 | awk '{print $NF}'`
	OUTZIP="${OUTPUT_DIR}/${RUN_ID}.zip"
	./testground collect --runner=local:docker --output=$OUTZIP $RUN_ID
	unzip $OUTZIP -d $OUTPUT_DIR
	cat ${OUTPUT_DIR}/${RUN_ID}/*/*/run.out | node $SCRIPT_DIR/aggregate.js $OUTFILE_CSV_BASE
done

node $SCRIPT_DIR/chart.js -d $RES_DIR -m time_to_fetch -b $BANDWIDTH_MB -l $LATENCY_MS -xlabel 'File size (MB)' -ylabel 'Time to fetch (s)' -xscale '9.53674316e-7' -yscale '1e-9'
gnuplot $RES_DIR/time_to_fetch.plot > $RES_DIR/time_to_fetch.svg
