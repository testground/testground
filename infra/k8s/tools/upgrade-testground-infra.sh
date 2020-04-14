#/usr/bin/env bash

# Updates sub-charts not managed by the testground team.

HERE=$(dirname $0)
pushd "$HERE"/testground-infra
helm dep build
helm upgrade --atomic --install testground-infra .
popd
