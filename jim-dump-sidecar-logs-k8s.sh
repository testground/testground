#! /bin/bash

for pod in `kubectl get pods | grep sidecar | awk '{print $1}'`; do
  echo $pod
  kubectl logs $pod | tail -10
  echo '---'
done
