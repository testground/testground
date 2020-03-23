#/usr/bin/env bash

# Updates sub-charts not managed by the testground team.

thirdparty=("stable/prometheus-pushgateway"
	    "bitnami/prometheus-operator"
	    "bitnami/redis")

for repo in "${thirdparty[@]}"
do
	helm pull "$repo" --untar --untardir ./testground-infra/charts/
done
