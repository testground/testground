#! /bin/bash

INSTANCES=$1

if [ -z "$INSTANCES" ]; then
  INSTANCES=3
fi

DIR=$(basename `pwd`)
NAME=$(echo $DIR | perl -p -e 's,20(\d\d)\.(\d\d)\.(\d\d),\1\2\3,')
echo $NAME

../../testground --vv run single $NAME/bootstrap \
    --builder=docker:lotus \
    --runner=local:docker \
    --run-cfg keep_containers=true \
    --run-cfg keep_service=true \
    --test-param keep-alive=true \
    --instances=$INSTANCES