#! /bin/bash

set -e
set +x

rm -rf extern
git submodule update

# source = "git+https://github.com/filecoin-project/rust-filecoin-proofs-api.git#b00d2e26c68e49b81e434739336383304b293395"
#PROOFS_API_HASH=$(sed -n 's,source = "git+https://github.com/filecoin-project/rust-filecoin-proofs-api.git#\(.*\)"$,\1,p' extern/filecoin-ffi/rust/Cargo.lock)
#echo rust-filecoin-proofs-api $PROOFS_API_HASH

cp extern/filecoin-ffi/rust/Cargo.toml extern/filecoin-ffi/rust/Cargo.toml.save
cp extern/filecoin-ffi/rust/Cargo.lock extern/filecoin-ffi/rust/Cargo.lock.save

if ! grep 'storage-proofs-core' extern/filecoin-ffi/rust/Cargo.toml; then
cat <<EOF >> extern/filecoin-ffi/rust/Cargo.toml
[patch.crates-io]
storage-proofs-core = { path = "../../rust-fil-proofs/storage-proofs/core" }

EOF
fi

#perl -pi -e 'BEGIN {undef $/} s#git = "https://github.com/filecoin-project/rust-filecoin-proofs-api.git"\nbranch = "master"#path = "../../rust-filecoin-proofs-api"#mg' extern/filecoin-ffi/rust/Cargo.toml
#diff -u extern/filecoin-ffi/rust/Cargo.toml.save extern/filecoin-ffi/rust/Cargo.toml || true

#git clone git@github.com:filecoin-project/rust-filecoin-proofs-api.git extern/rust-filecoin-proofs-api
#pushd extern/rust-filecoin-proofs-api
#git checkout $PROOFS_API_HASH
#popd

# git = "https://github.com/filecoin-project/rust-fil-proofs"
# rev = "d7896c29ef3c0cc8c04f9fab7ef434e6691ed480"
#PROOFS_HASH=
#perl -n -e 's#git = "https://github.com/filecoin-project/rust-fil-proofs"\nrev = "([a-f0-9]+)"#\1#mgp' extern/rust-filecoin-proofs-api/Cargo.toml
#perl -n -e 'BEGIN {undef $/} print if s#https://github.com/filecoin-project/rust-fil-proofs"\nrev = "([a-f0-9]+)"#\1#mgp' extern/rust-filecoin-proofs-api/Cargo.toml
#PROOFS_HASH=$(sed -n 's,rev = "\(.*\)",\1,p' extern/rust-filecoin-proofs-api/Cargo.toml)
#echo rust-fil-proofs $PROOFS_HASH

#cp extern/rust-filecoin-proofs-api/Cargo.toml extern/rust-filecoin-proofs-api/Cargo.toml.save
# Before:
# version = "1.0.0-alpha.0"
# git = "https://github.com/filecoin-project/rust-fil-proofs"
# rev = "d7896c29ef3c0cc8c04f9fab7ef434e6691ed480"
# After:
# path = "../rust-fil-proofs/filecoin-proofs"
#perl -pi -e 'BEGIN {undef $/} s#version = "1.0.0-alpha.0"\ngit = "https://github.com/filecoin-project/rust-fil-proofs"\nrev = "[a-f0-9]+"#path = "../rust-fil-proofs/filecoin-proofs"#mg' extern/rust-filecoin-proofs-api/Cargo.toml
#diff -u extern/rust-filecoin-proofs-api/Cargo.toml.save extern/rust-filecoin-proofs-api/Cargo.toml || true

git clone git@github.com:filecoin-project/rust-fil-proofs.git extern/rust-fil-proofs
pushd extern/rust-fil-proofs
#git checkout $PROOFS_HASH
git checkout storage-proofs-core-v4.0.0
cat ../../jim-proofs-flock.patch | patch -p1
popd

