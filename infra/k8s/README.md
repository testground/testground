# Running Testground with Kubernetes on Digitial Ocean backend

## Requirements

1. Digital Ocean account
2. [Docker Hub](https://hub.docker.com/) account
3. [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
4. Docker
5. [helm](https://github.com/helm/helm)

## Background

This document is explaining how to execute a Testground test plan locally, while Testground targets a Kubernetes cluster on Digital Ocean for execution of all tasks. Images are built locally by Testground via Docker, and pushed to Docker Hub. Redis is to be installed manually on Kubernetes, prior to running any test plans.

## Introduction

1. `kubectl` - `kubectl` is a command line interface for running commands against Kubernetes clusters. It sends commands to the Kubernetes master, and all `kubectl` commands are run on the developer's machine in this tutorial. `kubectl` has to be configured with access parameters so that it has access to any given Kubernetes cluster.

2. `helm` - `helm` is a package manager for Kubernetes. If you have configured `kubectl` to a given cluster, then `helm` also has access to it, and you can run `helm` commands from your local machine towards it.

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

## Destroying the cluster

Do not forget to delete the cluster on Digital Ocean once you are done running test plans.

## Known issues and future improvements

- [ ] 1. Kubernetes cluster creation - we intend to automate this, so that it is one command in the future, most probably with `terraform`.

- [ ] 2. Testground dependencies - we intend to automate this, so that all dependencies for Testground are installed with one command, or as a follow-up provisioner on `terraform` - such as `redis`, `filebeat`, maybe `testground daemon`, etc.

- [ ] 3. Alerts (and maybe auto-scaling down) for idle clusters, so that we don't incur costs.
