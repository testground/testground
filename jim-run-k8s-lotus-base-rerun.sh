#! /bin/bash

TAG=$1

if [ -z "$TAG" ]; then
  TAG=$(cat LAST-TAG)
  echo "Using tag $TAG"
fi

./testground --vv run single lotus-base/upload \
    --builder=docker:lotus \
    --runner=cluster:k8s \
    --build-cfg push_registry=true \
    --build-cfg registry_type=aws \
    --run-cfg keep_service=true \
    --instances=3 \
    --use-build 909427826938.dkr.ecr.us-west-2.amazonaws.com/testground-us-west-2-lotus-base:$TAG
