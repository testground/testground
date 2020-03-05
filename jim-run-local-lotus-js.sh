#! /bin/bash

INSTANCES=$1

if [ -z "$INSTANCES" ]; then
  INSTANCES=2
fi

./testground --vv run single lotus-js/bootstrap \
    --builder=docker:lotus \
    --runner=local:docker \
    --run-cfg keep_service=true \
    --instances=$INSTANCES
