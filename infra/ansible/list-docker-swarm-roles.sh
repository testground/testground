#! /bin/bash

NODES=$(docker node ls | awk '{ print $1 }' | grep -v '^ID$')

for n in $NODES; do
	docker node inspect $n | \
	jq -r '[.[0].ID, .[0].Description.Hostname, .[0].Spec.Labels.TGRole] | join(" ")'
done
