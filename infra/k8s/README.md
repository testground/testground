# Running Testground with Kubernetes on Digitial Ocean backend

## Requirements

1. Digital Ocean account
2. [Docker Hub](https://hub.docker.com/) account
3. [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
4. Docker
5. [helm](https://github.com/helm/helm)

## Background

This document is explaining how to execute a Testground test plan locally, while Testground targets a Kubernetes cluster on Digital Ocean for execution of all tasks. Images are built locally by Testground via Docker, and pushed to Docker Hub. Redis is to be installed manually on Kubernetes, prior to running any test plans.

## Setup Kubernetes Cluster on Digital Ocean

1. Create a Kubernetes cluster on Digital Ocean, with the necessary number of workers.

Note: There is a limit of 100 pods per worker, so calculate the number of workers you need accordingly.

2. Configure your `kubectl` context and authentication with Digital Ocean. Confirm that `kubectl get nodes` shows the number of nodes in your cluster.

```
$  kubectl get nodes
NAME                                                STATUS   ROLES    AGE     VERSION
gke-standard-cluster-1-default-pool-bce3c7de-0f20   Ready    <none>   4h46m   v1.14.8-gke.17
gke-standard-cluster-1-default-pool-bce3c7de-b623   Ready    <none>   4h46m   v1.14.8-gke.17
gke-standard-cluster-1-default-pool-bce3c7de-btmc   Ready    <none>   4h46m   v1.14.8-gke.17
gke-standard-cluster-1-default-pool-bce3c7de-gprr   Ready    <none>   4h47m   v1.14.8-gke.17
gke-standard-cluster-1-default-pool-bce3c7de-sbdv   Ready    <none>   4h46m   v1.14.8-gke.17
```

## Setup Testground remote dependencies on your cluster

1. Create a `Redis` service on your Kubernetes cluster

```
helm install redis stable/redis --values redis-values.yaml
```

## Configure Testground to push the built images to Docker Hub

1. Edit your `.env.toml` and add credentials for your Docker Hub account where the ready images will be pushed to. You will need an access token from Docker Hub for that step.

## Run a Testground testplan

```
$  testground -vv run dht/find-peers \
    --builder=docker:go \
    --runner=cluster:k8s \
    --build-cfg bypass_cache=true \
    --build-cfg push_registry=true \
    --build-cfg registry_type=dockerhub \
    --run-cfg keep_service=true \
    --instances=328
```
