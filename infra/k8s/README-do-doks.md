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

2. Configure your `kubectl` context and authentication with Digital Ocean. An easy way to do it is to download the config provided by Digital Ocean and placing it at `~/.kube/config`. Confirm that `kubectl get nodes` shows the number of nodes in your cluster.

```
$  kubectl get nodes
NAME                  STATUS   ROLES    AGE   VERSION
pool-jyub1rb95-snl2   Ready    <none>   60m   v1.16.2
pool-jyub1rb95-snl5   Ready    <none>   60m   v1.16.2
pool-jyub1rb95-snl6   Ready    <none>   60m   v1.16.2
```

## Setup Testground remote dependencies on your cluster

1. Create a `Redis` service on your Kubernetes cluster

```
helm install redis stable/redis --values redis-values.yaml
```

2. Create a `Sidecar` service on your Kubernetes cluster.

```
kubectl apply -f sidecar.yaml
```

## Configure Testground to push the built images to Docker Hub

1. Edit your `.env.toml` and add credentials for your Docker Hub account where the ready images will be pushed to. You will need an access token from Docker Hub for that step.

## Run a Testground testplan

```
testground -vv run dht/find-peers \
    --builder=docker:go \
    --runner=cluster:k8s \
    --build-cfg bypass_cache=true \
    --build-cfg push_registry=true \
    --build-cfg registry_type=dockerhub \
    --run-cfg keep_service=true \
    --instances=16
```

## Destroying the cluster

Do not forget to delete the cluster on Digital Ocean once you are done running test plans.

## Cleanup after Testground and other useful commands

Testground is still in very early stage of development. It is possible that it crashes, or doesn't properly clean-up after a testplan run. Here are a few commands that could be helpful for you to inspect the state of your Kubernetes cluster and clean up after Testground.

- `kubectl delete pods -l testground.plan=dht --grace-period=0` - delete all pods that have the `testground.plan=dht` label

- `kubectl delete job <job-id, e.g. tg-dht-find-peers-e47e5301-d6f7-4ded-98e8-b2d3dc60a7bb>` - delete a specific job

- `kubectl get pods -o wide` - get all pods

- `kubectl logs -f <pod-id, e.g. tg-dht-c95b5>` - follow logs from a given pod

## Known issues and future improvements

- [ ] 1. Kubernetes cluster creation - we intend to automate this, so that it is one command in the future, most probably with `terraform`.

- [ ] 2. Testground dependencies - we intend to automate this, so that all dependencies for Testground are installed with one command, or as a follow-up provisioner on `terraform` - such as `redis`, `filebeat`, maybe `testground daemon`, etc.

- [ ] 3. Alerts (and maybe auto-scaling down) for idle clusters, so that we don't incur costs.

- [ ] 4. Sidecar currently has a hard-coded value for the Redis service endpoint. We need to fix this and release a new version.

- [ ] 5. We need to decide where Testground is going to publish built docker images when used with Digitial Ocean - DockerHub? or? This might incur a lot of costs if you build a large image and download it from 100 VMs repeatedly.
