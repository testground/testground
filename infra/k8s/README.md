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

We configure Weave to use 16.0.0.0/4 as CIDR (we want to test `libp2p` nodes with IPs in public ranges), so this is the CIDR for the Testground `data` network. The `sidecar` is responsible for setting up the `data` network for every testplan instance.

In order to have two different networks attached to pods in Kubernetes, we run the [CNI-Genie CNI](https://github.com/cni-genie/CNI-Genie).


## Requirements

1. [kops](https://github.com/kubernetes/kops/releases). >= 1.17.0-alpha.1
2. [AWS CLI](https://aws.amazon.com/cli)
3. [helm](https://github.com/helm/helm)

## Set up cloud credentials, cluster specification and repositories for dependencies

1. [Configure your AWS credentials](https://docs.aws.amazon.com/cli/)

2. Download shared key for `kops`. We use a shared key, so that everyone on the team can log into any cluster and have full access.

```
aws s3 cp s3://kops-shared-key-bucket/testground_rsa ~/.ssh/
aws s3 cp s3://kops-shared-key-bucket/testground_rsa.pub ~/.ssh/
chmod 700 ~/.ssh/testground_rsa
```

Or generate your own key, for example

```
ssh-keygen -t rsa -b 4096 -C "your_email@example.com"
```

3. Create a bucket for `kops` state. This is similar to Terraform state bucket.

```
aws s3api create-bucket \
    --bucket kops-backend-bucket \
    --region eu-central-1 --create-bucket-configuration LocationConstraint=eu-central-1
```

4. Pick up
- a cluster name,
- set AWS zone
- set `kops` state store bucket
- set number of worker nodes
- set location for cluster spec to be generated
- set location of your cluster SSH public key
- set credentials and locations for `outputs` S3 bucket

You might want to add them to your `rc` file (`.zshrc`, `.bashrc`, etc.)

```
export NAME=<desired kubernetes cluster name>
export KOPS_STATE_STORE=s3://<kops state s3 bucket>
export ZONE=<aws region, for example eu-central-1a>
export WORKER_NODES=4
export PUBKEY=~/.ssh/testground_rsa.pub

# details for S3 bucket to be used for assets
export ASSETS_BUCKET_NAME=$(aws s3 cp s3://assets-s3-bucket-credentials/assets_bucket_name -)
export ASSETS_ACCESS_KEY=$(aws s3 cp s3://assets-s3-bucket-credentials/assets_access_key -)
export ASSETS_SECRET_KEY=$(aws s3 cp s3://assets-s3-bucket-credentials/assets_secret_key -)

# depends on region, for example "https://s3.eu-central-1.amazonaws.com:443"
export ASSETS_S3_ENDPOINT=$(aws s3 cp s3://assets-s3-bucket-credentials/assets_s3_endpoint -)
```

5. Set up Helm and add the `stable` Helm Charts repository

```
helm repo add stable https://kubernetes-charts.storage.googleapis.com/
helm repo update
```

## Install the kuberntes cluster

For example, to create a monitored cluster in the region specified in $ZONE with $WORKER_NODES number of workers:

```
./install.sh ./cluster.yaml
```


## Destroy the cluster when you're done working on it

```
kops delete cluster $NAME --yes
```


## Configure and run your Testground daemon

```
testground --vv daemon
```


## Run a Testground testplan

Use compositions: [/docs/COMPOSITIONS.md](../../docs/COMPOSITIONS.md).

or

```
testground --vv run single network/ping-pong \
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
testground --vv run single dht/find-peers \
    --builder=docker:go \
    --runner=cluster:k8s \
    --build-cfg push_registry=true \
    --build-cfg registry_type=aws \
    --run-cfg keep_service=true \
    --instances=16
```

## Resizing the cluster

1. Edit the cluster state and change number of nodes.

```
kops edit ig nodes
```

2. Apply the new configuration
```
kops update cluster $NAME --yes
```

3. Wait for nodes to come up and for DaemonSets to be Running on all new nodes
```
watch 'kubectl get pods'
```

## Destroying the cluster

Do not forget to delete the cluster once you are done running test plans.


## Cleanup after Testground and other useful commands

Testground is still in very early stage of development. It is possible that it crashes, or doesn't properly clean-up after a testplan run. Here are a few commands that could be helpful for you to inspect the state of your Kubernetes cluster and clean up after Testground.

1. Delete all pods that have the `testground.plan=dht` label (in case you used the `--run-cfg keep_service=true` setting on Testground.
```
kubectl delete pods -l testground.plan=dht --grace-period=0 --force
```

2. Restart the `sidecar` daemon which manages networks for all testplans
```
kubectl delete pods -l name=testground-sidecar --grace-period=0 --force
```

3. Review all running pods
```
kubectl get pods -o wide
```

4. Get logs from a given pod
```
kubectl logs <pod-id, e.g. tg-dht-c95b5>
```

5. Check on the monitoring infrastructure (it runs in the monitoring namespace)
```
kubectl get pods --namespace monitoring
```

6. Get access to the Redis shell
```
kubectl port-forward svc/redis-master 6379:6379 &
redis-cli -h localhost -p 6379
```

7. Get access to the kubernetes dashboard 
```
kubectl proxy
```
and then, direct your browser to `http://localhost:8001/ui`



## Use a Kubernetes context for another cluster

`kops` lets you download the entire Kubernetes context config.

If you want to let other people on your team connect to your Kubernetes cluster, you need to give them the information.

```
kops export kubecfg --state $KOPS_STATE_STORE --name=$NAME
```

## Known issues and future improvements

- [ ] 1. Kubernetes cluster creation - we intend to automate this, so that it is one command in the future, most probably with `terraform`.

- [ ] 2. Testground dependencies - we intend to automate this, so that all dependencies for Testground are installed with one command, or as a follow-up provisioner on `terraform` - such as `redis`, `sidecar`, etc.

- [ ] 3. Alerts (and maybe auto-scaling down) for idle clusters, so that we don't incur costs.

- [X] 4. We need to decide where Testground is going to publish built docker images - DockerHub? or? This might incur a lot of costs if you build a large image and download it from 100 VMs repeatedly.
Resolution: For now we are using AWS ECR, as clusters are also on AWS.
