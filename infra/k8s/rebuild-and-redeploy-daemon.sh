#!/bin/bash

set -o errexit
set -o pipefail

set -e

err_report() {
    echo "Error on line $1"
}

trap 'err_report $LINENO' ERR

pushd ../../

docker build -t nonsens3/testground:daemon -f Dockerfile.daemon .

docker push nonsens3/testground:daemon

popd

pushd testground-daemon

kubectl delete deployment testground-daemon || true
kubectl delete service testground-daemon || true
kubectl apply -f deployment.yaml -f service.yaml

popd
