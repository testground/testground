#! /bin/bash

watch -n 5 "lotus chain list --count=1; lotus-storage-miner info; echo --- Node; cat /outputs/node.out | grep -a -v 'New heaviest\|insecure test\|new block\|New block\|scheduling incoming' | tail -10 ; echo --- Miner; cat /outputs/miner.out | tail -10"

