# Tools and commands that are helpful during running Testground on Kubernetes

1. `confirm_dns_route.sh`

A command which makes sure that a DNS route has been setup on all `host` machines. Number of lines returned should be the number of nodes in the cluster.

```
./confirm_dns_route.sh | wc -l
```
