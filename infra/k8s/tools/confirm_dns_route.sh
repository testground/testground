#!/bin/zsh

IFS=$'\n' sidecars=( $(kubectl get pods | grep sidecar | awk '{print $1}') )
for i in "${sidecars[@]}"; do
  kubectl -n kube-system logs $i -c iproute-add | grep 100.64
done
