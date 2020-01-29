# Setting up a self-managed Kubernetes cluster with kops on AWS for Testground

In this directory, you will find:

```
» tree
.
├── README.md
└── kops-weave                     # Kubernetes resources for setting up networking with Weave and Flannel
```

## Introduction

Kubernetes Operations (kops) is a tool which helps to create, destroy, upgrade and maintain production-grade Kubernetes clusters from the command line. We use it to create a k8s cluster on AWS.

We use CoreOS Flannel for networking on Kubernetes - for the default Kubernetes network, which in Testground terms is called the `control` network.

We use Weave for the `data` plane on Testground - a secondary overlay network that we attach containers to on-demand.

`kops` uses 100.96.0.0/11 for pod CIDR range, so this is what we use for the `control` network.

Weave by default uses 10.32.0.0/11 as CIDR, so this is the CIDR for the Testground `data` network. The `sidecar` is responsible for setting up the `data` network for every testplan instance.

In order to have two different networks attached to pods in Kubernetes, we run the [CNI-Genie CNI](https://github.com/cni-genie/CNI-Genie).


## Requirements

1. [kops](https://github.com/kubernetes/kops/releases). >= 1.17.0-alpha.1
2. [AWS CLI](https://aws.amazon.com/cli)
3. [helm](https://github.com/helm/helm)

## Set up infrastructure with kops

1. [Configure your AWS credentials](https://docs.aws.amazon.com/cli/)

2. Create a bucket for `kops` state. This is similar to Terraform state bucket.

```
aws s3api create-bucket \
    --bucket kops-backend-bucket \
    --region eu-central-1 --create-bucket-configuration LocationConstraint=eu-central-1
```

3. Pick up a cluster name, and set zone and kops state store

```
export NAME=my-first-cluster-kops.k8s.local
export KOPS_STATE_STORE=s3://kops-backend-bucket
export ZONES=eu-central-1a
```

4. Generate the cluster spec

```
kops create cluster \
  --zones $ZONES \
  --master-zones $ZONES \
  --master-size c5.2xlarge \
  --node-size c5.2xlarge \
  --node-count 8 \
  --networking flannel \
  --name $NAME \
  --dry-run \
  -o yaml > cluster.yaml
```

5. Update `kubelet` section in spec with:
```
  kubelet:
    anonymousAuth: false
    maxPods: 200
    allowedUnsafeSysctls:
    - net.core.somaxconn
```

6. Create cluster
```
kops create -f cluster.yaml
kops create secret --name $NAME sshpublickey admin -i ~/.ssh/id_rsa.pub
kops update cluster $NAME --yes
```

7. Wait for all nodes to appear in `kubectl get nodes` with `Ready` state, and for all pods in `kube-system` namespace to be `Running`.
```
watch 'kubectl get nodes -o wide'

kubectl -n kube-system get pods -o wide
```

8. Install CNI-Genie, Weave and Dummy daemonset - we need a container on every worker node so that interface `cni0` is created, and Weave's initContainer can add a route to the Services CIDR

```
kubectl apply -f ./infra/k8s/kops-weave/genie-plugin.yaml \
              -f ./infra/k8s/kops-weave/dummy.yml \
              -f ./infra/k8s/kops-weave/weave.yml \
              -f ./infra/k8s/kops-weave/s3bucket.yml
```

9. Destroy the cluster when you're done working on it

```
kops delete cluster $NAME --yes
```


## Setup Testground remote dependencies on your cluster

1. Set up Helm

```
helm repo add stable https://kubernetes-charts.storage.googleapis.com/
helm repo update
```

2. Create a `Redis` service on your Kubernetes cluster

```
helm install redis stable/redis --values ./infra/k8s/redis-values.yaml
```

3. Wait for `Redis` to be `Ready 1/1`

4. Create a `Sidecar` service on your Kubernetes cluster.

```
kubectl apply -f ./infra/k8s/sidecar.yaml
```


## Configure and run your Testground daemon

```
testground --vv daemon
```


## Run a Testground testplan

```
testground --vv run network/ping-pong \
    --builder=docker:go \
    --runner=cluster:k8s \
    --build-cfg bypass_cache=true \
    --build-cfg push_registry=true \
    --build-cfg registry_type=aws \
    --run-cfg keep_service=true \
    --instances=2
```

or

```
testground --vv run dht/find-peers \
    --builder=docker:go \
    --runner=cluster:k8s \
    --build-cfg push_registry=true \
    --build-cfg registry_type=aws \
    --run-cfg keep_service=true \
    --instances=16
```

## Destroying the cluster

Do not forget to delete the cluster once you are done running test plans.


## Cleanup after Testground and other useful commands

Testground is still in very early stage of development. It is possible that it crashes, or doesn't properly clean-up after a testplan run. Here are a few commands that could be helpful for you to inspect the state of your Kubernetes cluster and clean up after Testground.

- `kubectl delete pods -l testground.plan=dht --grace-period=0` - delete all pods that have the `testground.plan=dht` label

- `kubectl delete job <job-id, e.g. tg-dht-find-peers-e47e5301-d6f7-4ded-98e8-b2d3dc60a7bb>` - delete a specific job

- `kubectl get pods -o wide` - get all pods

- `kubectl logs -f <pod-id, e.g. tg-dht-c95b5>` - follow logs from a given pod


## Known issues and future improvements

- [ ] 1. Kubernetes cluster creation - we intend to automate this, so that it is one command in the future, most probably with `terraform`.

- [ ] 2. Testground dependencies - we intend to automate this, so that all dependencies for Testground are installed with one command, or as a follow-up provisioner on `terraform` - such as `redis`, `sidecar`, etc.

- [ ] 3. Alerts (and maybe auto-scaling down) for idle clusters, so that we don't incur costs.

- [ ] 4. We need to decide where Testground is going to publish built docker images - DockerHub? or? This might incur a lot of costs if you build a large image and download it from 100 VMs repeatedly.
