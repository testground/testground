#! /bin/bash

DIR=$(basename `pwd`)

../../testground --vv build single $DIR/placeholder \
    --builder=docker:powergate
