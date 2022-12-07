#!/usr/bin/env sh
set -e
set -o pipefail
unameOut="$(uname -s)"
TESTGROUND_HOME="$HOME/.config/testground"
case "${unameOut}" in
    Darwin*)    TESTGROUND_HOME="$HOME/Library/Application Support";;
esac

mkdir -p "$TESTGROUND_HOME" && cd "$TESTGROUND_HOME"

docker pull iptestground/testground:edge
docker pull iptestground/sync-service:edge
docker pull iptestground/sidecar:edge

# At the moment this is the fastest way to get a pre-built testground binary.
docker run -v ${PWD}:/mount --rm --entrypoint cp iptestground/testground:edge /testground /mount/testground

if [ -z $GITHUB_PATH ]; then
    echo "Testground was installed to \"$TESTGROUND_HOME\". Add this to your path."
else
    echo "$TESTGROUND_HOME" >> $GITHUB_PATH
fi
