#!/usr/bin/env sh
set -e
set -o pipefail

docker pull iptestground/testground:edge
docker pull iptestground/sync-service:edge
docker pull iptestground/sidecar:edge

# At the moment this is the fastest way to get a pre-built testground binary.
id="$(docker create iptestground/testground:edge)"
docker cp $id:/testground $PWD/testground
docker rm $id

if [ -z $GITHUB_PATH ]; then
    echo "Testground was installed to \"$PWD\". Add this to your path."
else
    echo "$PWD" >> $GITHUB_PATH
fi
