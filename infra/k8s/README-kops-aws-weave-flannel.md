# Setting up a self-managed Kubernetes cluster with kops on AWS for Testground

In this directory, you will find:

```
» tree
.
├── README-kops-aws.md
└── kops-weave                     # Kubernetes resources for setting up networking with Weave and Flannel
```

## Introduction

Kubernetes Operations (kops) is a tool which helps to create, destroy, upgrade and maintain production-grade Kubernetes clusters from the command line. We use it to create a k8s cluster on AWS.

We use CoreOS Flannel for networking on Kubernetes - for the default Kubernetes network

We use Weave For a secondary overlay network that pods can attach to on-demand

kops uses 100.96.0.0/11 for pod CIDR range, so this is what we use for the default Kubernetes network.

We use 10.32.0.0/11 for the secondary overlay network with Weave.

In order to have two different networks attached to pods in Kubernetes, we also need to run the [Multus CNI](https://github.com/intel/multus-cni), or the [CNI-Genie CNI](https://github.com/cni-genie/CNI-Genie). In this tutorial we use CNI-Genie.


## Requirements

- 1. [kops](https://github.com/kubernetes/kops/releases). >= 1.17.0-alpha.1


## Set up infrastructure with kops

1. [Configure your AWS credentials](https://docs.aws.amazon.com/cli/)

2. Create a bucket for kops state. This is similar to Terraform state bucket.

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

4. Create The cluster

```
kops create cluster \
  --zones $ZONES \
  --master-zones $ZONES \
  --master-size m4.xlarge \
  --node-size m4.xlarge \
  --node-count 2 \
  --networking cni \
  --name $NAME \
  --yes
```

5. Wait for `master` node to appear in `kubectl get nodes` - it will be in `NotReady` state as we haven't installed any Networking CNI yet.

6. Install Flannel

```
kubectl apply -f ./flannel.yml
```

7. Wait for all nodes to appear in `kubectl get nodes` with `Ready` state.

8. Install CNI-Genie

```
kubectl apply -f ./genie-plugin.yaml
```

9. Install Weave

```
kubectl apply -f ./weave.yml
```

10. Create a sample pod and attach it to both networks

```
kubectl apply -f pod1.yml
kubectl apply -f pod2.yml
```

11. Exec into the containers

```
kubectl exec -it pod1 sh
kubectl exec -it pod2 sh
```


12. Ping interfaces

```
ifconfig
ping
```

13. Destroy the cluster when you're done working on it

```
kops delete cluster $NAME --yes
```


## Known issues

1. When pods are started without `annotations.cni` they are not always attached to the Flannel network by default - you have to explicitly set an annotation. It is not clear why at this point.
