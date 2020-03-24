#! /bin/bash

INSTANCES=$1

if [ -z "$INSTANCES" ]; then
  INSTANCES=3
fi

../../testground --vv run single lotus-js-testnet-v3-200323/bootstrap \
    --builder=docker:lotus \
    --runner=cluster:k8s \
    --build-cfg push_registry=true \
    --build-cfg registry_type=aws \
    --run-cfg keep_service=true \
    --test-param keep-alive=true \
    --test-param ssh-tunnel=tg-lotus \
    --instances=$INSTANCES
