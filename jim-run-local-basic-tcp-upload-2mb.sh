#! /bin/bash

INSTANCES=$1

if [ -z "$INSTANCES" ]; then
  INSTANCES=2
fi

./testground --vv run single basic-tcp/upload \
    --builder=docker:go \
    --runner=local:docker \
    --run-cfg keep_service=true \
    --test-param size=2MB \
    --instances=$INSTANCES
