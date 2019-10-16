#! /bin/bash

echo "Install IPFS: $1"

cd /tmp
wget https://dist.ipfs.io/go-ipfs/v${GO_IPFS_VERSION}/go-ipfs_v${GO_IPFS_VERSION}_linux-amd64.tar.gz
tar xf go-ipfs_v${GO_IPFS_VERSION}_linux-amd64.tar.gz

