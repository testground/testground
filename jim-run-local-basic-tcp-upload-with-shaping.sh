#! /bin/bash

INSTANCES=$1

if [ -z "$INSTANCES" ]; then
  INSTANCES=2
fi

./testground --vv run single basic-tcp/upload-with-shaping \
    --builder=docker:go \
    --runner=local:docker \
    --run-cfg keep_service=true \
    --test-param bandwidth=512k \
    --test-param delay-between-uploads=8000 \
    --instances=$INSTANCES
