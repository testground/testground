#! /bin/bash

./testground --vv run single network/ping-pong \
    --builder=docker:go \
    --runner=local:docker \
    --run-cfg keep_service=true \
    --instances=2
