# Tools and commands that are helpful during running Testground on Kubernetes

1. `confirm_weave.sh`

A command which makes sure that `weave` has setup routes on `host` machines correctly. Lines returned should be the number of nodes in the cluster.

```
confirm_weave | wc -l
```
