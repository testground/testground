#! /bin/bash

./testground --vv build single lotus-base/placeholder \
    --build-cfg lotus_path=../lotus-testnet-v3 \
    --builder=docker:lotus
