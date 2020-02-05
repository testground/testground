#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset

START_TIME=`date +%s`

echo "Creating cluster for Testground..."
echo

NAME=$1
CLUSTER_SPEC=$2
PUBKEY=$3
WORKER_NODES=$4

echo "Name: $NAME"
echo "Cluster spec: $CLUSTER_SPEC"
echo "Public key: $PUBKEY"
echo "Worker nodes: $WORKER_NODES"
echo

ASSETS_BUCKET_NAME=$(aws s3 cp s3://assets-s3-bucket-credentials/assets_bucket_name -)
ASSETS_ACCESS_KEY=$(aws s3 cp s3://assets-s3-bucket-credentials/assets_access_key -)
ASSETS_SECRET_KEY=$(aws s3 cp s3://assets-s3-bucket-credentials/assets_secret_key -)

kops create -f $CLUSTER_SPEC
kops create secret --name $NAME sshpublickey admin -i $PUBKEY
kops update cluster $NAME --yes

## wait for worker nodes and master to be ready
echo "Wait for Cluster nodes to be Ready..."
echo
READY_NODES=0
while [ "$READY_NODES" -ne $(($WORKER_NODES + 1)) ]; do READY_NODES=$(kubectl get nodes 2>/dev/null | grep -v NotReady | grep Ready | wc -l || true); echo "Got $READY_NODES ready nodes"; sleep 5; done;

echo "Cluster nodes are Ready"
echo

echo "Add secret for S3 bucket"
echo
kubectl create secret generic assets-s3-bucket --from-literal=access-key="$ASSETS_ACCESS_KEY" \
                                               --from-literal=secret-key="$ASSETS_SECRET_KEY" \
                                               --from-literal=bucket-name="$ASSETS_BUCKET_NAME"


echo "Install Weave, CNI-Genie, s3bucket DaemonSet, Sidecar Daemonset..."
echo
kubectl apply -f ./kops-weave/genie-plugin.yaml \
              -f ./kops-weave/weave.yml \
              -f ./kops-weave/s3bucket.yml \
              -f ./sidecar.yaml

echo "Install Redis..."
echo
helm install redis stable/redis --values ./redis-values.yaml


echo "Wait for Sidecar to be Ready..."
echo
RUNNING_SIDECARS=0
while [ "$RUNNING_SIDECARS" -ne "$WORKER_NODES" ]; do RUNNING_SIDECARS=$(kubectl get pods | grep testground-sidecar | grep Running | wc -l || true); echo "Got $RUNNING_SIDECARS running sidecar pods"; sleep 5; done;

echo "Testground cluster is ready"
echo

END_TIME=`date +%s`
echo "Execution time was `expr $END_TIME - $START_TIME` seconds"
