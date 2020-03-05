#! /bin/bash

kubectl get pods | grep ^tg- | awk '{print $1}' | xargs kubectl delete pod

