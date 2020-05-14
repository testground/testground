#! /bin/bash

tail -f /outputs/miner.out | grep -v 'Time delta\|Generate candidates\|Generating fake\|mined new block'
