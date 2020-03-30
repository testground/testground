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
    --runner=cluster:k8s \
    --build-cfg push_registry=true \
    --build-cfg registry_type=aws \
    --run-cfg keep_service=true \
    --test-param keep-alive=true \
    --test-param ssh-tunnel=tg-lotus \
    --instances=$INSTANCES
