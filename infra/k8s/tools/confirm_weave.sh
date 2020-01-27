#!/bin/zsh

IFS=$'\n' weaves=( $(kubectl -n kube-system get pods | grep weave | awk '{print $1}') )
for i in "${weaves[@]}"; do
  kubectl -n kube-system logs $i -c iproute-add | grep 100.64
done
