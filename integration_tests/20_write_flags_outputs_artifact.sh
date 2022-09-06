#!/bin/bash
my_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$my_dir/header.sh"

testground plan import --from ./plans --name testground

cp ./plans/_integrations/issue-tbd-allow-files-overwrite.toml "$TEMPDIR/original.toml"
cp ./plans/_integrations/issue-tbd-allow-files-overwrite.toml "$TEMPDIR/source.toml"

pushd $TEMPDIR

testground build composition \
    --file ./source.toml \
    --output-composition ./output.toml \
    --wait | tee build.out;
export ARTIFACT=$(awk -F\" '/generated build artifact/ {print $8}' build.out)

cmp --silent ./original.toml ./source.toml # the source was not changed
grep "$ARTIFACT" ./output.toml # the output contains our artifacts

popd

echo "terminating remaining containers"
testground terminate --builder docker:go
