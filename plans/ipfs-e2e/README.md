[issue](https://github.com/ipfs/testground/issues/465)


    Create several nodes running the old DHT with the old Bitswap
    Create several nodes running the new DHT with the new Bitswap
    Nodes should have realistic latency / bandwidth that may be expected for internet nodes
    Seed data of the following sizes on 1, 2 (1 old / 1 new) and 4 nodes (2 old / 2 new):
    1MB, 2MB, 4MB, 8MB, 16MB, 32MB, 64MB, 128MB
    Fetch data of each size using
        new node as a leech
        old node as a leech

