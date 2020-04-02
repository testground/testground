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

1. [kops](https://github.com/kubernetes/kops/releases) >= 1.17.0-alpha.1
2. [terraform](https://terraform.io) >= 0.12.21
3. [AWS CLI](https://aws.amazon.com/cli)
4. [helm](https://github.com/helm/helm) >= 3.0

## Set up cloud credentials, cluster specification and repositories for dependencies

1. [Generate your AWS IAM credentials](https://console.aws.amazon.com/iam/home#/security_credentials).

    * [Configure the aws-cli tool with your credentials](https://docs.aws.amazon.com/cli/).
    * Create a `.env.toml` file (copying over the [`env-example.toml`](https://github.com/ipfs/testground/blob/master/env-example.toml) at the root of this repo as a template), and add your region to the `[aws]` section.

2. Download shared key for `kops`. We use a shared key, so that everyone on the team can log into any cluster and have full access.

```sh
$ aws s3 cp s3://kops-shared-key-bucket/testground_rsa ~/.ssh/
$ aws s3 cp s3://kops-shared-key-bucket/testground_rsa.pub ~/.ssh/
$ chmod 700 ~/.ssh/testground_rsa
```

Or generate your own key, for example

```sh
$ ssh-keygen -t rsa -b 4096 -C "your_email@example.com"
```

3. Create a bucket for `kops` state. This is similar to Terraform state bucket.

```sh
$ aws s3api create-bucket \
      --bucket <bucket_name> \
      --region <region> --create-bucket-configuration LocationConstraint=<region>
```

Where:

* `<bucket_name>` is a unique AWS account-wide unique bucket name to store this cluster's kops state, e.g. `kops-backend-bucket-<your_username>`.
* `<region>` is an AWS region like `eu-central-1` or `us-west-2`.

4. Pick:

- a cluster name,
- set AWS region
- set AWS availability zone (not region; this is something like `us-west-2a` [availability zone], not `us-west-2` \[region])
- set `kops` state store bucket
- set number of worker nodes
- set location for cluster spec to be generated
- set location of your cluster SSH public key
- set credentials and locations for `outputs` S3 bucket

You might want to add them to your `rc` file (`.zshrc`, `.bashrc`, etc.), or to an `.env.sh` file that you source.

In addition to the initial cluster setup, these variables should be accessible to the daemon. If these variables are
manually set or you source them manually, you should make sure to do so before starting the testground daemon.

```sh
# NAME needs be a subdomain of an existing Route53 domain name. The testground team uses .testground.ipfs.team, which is set up by our Terraform configs.
export NAME=<desired kubernetes cluster name (cluster name must be a fully-qualified DNS name (e.g. mycluster.myzone.com)>
export KOPS_STATE_STORE=s3://<kops state s3 bucket>
export AWS_REGION=<aws region, for example eu-central-1>
export ZONE=<aws availability zone, for example eu-central-1a>
export WORKER_NODES=4
export PUBKEY=$HOME/.ssh/testground_rsa.pub
```

5. Set up Helm and add the `stable` Helm Charts repository

If you haven't, [install helm now](https://helm.sh/docs/intro/install/).

```sh
$ helm repo add stable https://kubernetes-charts.storage.googleapis.com/
$ helm repo add bitnami https://charts.bitnami.com/bitnami
$ helm repo update
```

## Install the Kubernetes cluster

To create a monitored cluster in the region specified in `$ZONE` with
`$WORKER_NODES` number of workers:

```sh
$ cd <testground_repo>/infra/k8s
$ ./install.sh ./cluster.yaml
```

If you're using the fish shell, you will want to summon bash:

```sh
$ cd <testground_repo>/infra/k8s
$ bash -c './install.sh ./cluster.yaml'
```

## Destroy the cluster when you're done working on it

```sh
$ ./delete.sh
```

## Configure and run your Testground daemon

```sh
$ cd <testground_repo>
$ go build .
$ ./testground --vv daemon
```

## Run a Testground testplan

Use compositions: [/docs/COMPOSITIONS.md](../../docs/COMPOSITIONS.md).

or

```sh
$ ./testground --vv run single network/ping-pong \
      --builder=docker:go \
      --runner=cluster:k8s \
      --build-cfg bypass_cache=true \
      --build-cfg push_registry=true \
      --build-cfg registry_type=aws \
      --run-cfg keep_service=true \
      --instances=2
```

or

```sh
$ ./testground --vv run single dht/find-peers \
      --builder=docker:go \
      --runner=cluster:k8s \
      --build-cfg push_registry=true \
      --build-cfg registry_type=aws \
      --run-cfg keep_service=true \
      --instances=16
```

## Resizing the cluster

1. Edit the cluster state and change number of nodes.

```sh
$ kops edit ig nodes
```

2. Apply the new configuration

```sh
$ kops update cluster $NAME --yes
```

3. Wait for nodes to come up and for DaemonSets to be Running on all new nodes

```sh
$ watch 'kubectl get pods'
```

## Destroying the cluster

Do not forget to delete the cluster once you are done running test plans.

## Testground observability

1. Access to Grafana (initial credentials are `username: admin` ; `password: testground`):

```sh
$ kubectl port-forward service/testground-infra-grafana 3000:80
```

2. Access the Prometheus Web UI

```sh
$ kubectl port-forward service/testground-infra-prometheu-prometheus 9090:9090
```

Direct your web browser to [http://localhost:9090](http://localhost:9090).

## Cleanup after Testground and other useful commands

Testground is still in very early stage of development. It is possible that it crashes, or doesn't properly clean-up after a testplan run. Here are a few commands that could be helpful for you to inspect the state of your Kubernetes cluster and clean up after Testground.

1. Delete all pods that have the `testground.plan=dht` label (in case you used the `--run-cfg keep_service=true` setting on Testground.

```sh
$ kubectl delete pods -l testground.plan=dht --grace-period=0 --force
```

2. Restart the `sidecar` daemon which manages networks for all testplans

```sh
$ kubectl delete pods -l name=testground-sidecar --grace-period=0 --force
```

3. Review all running pods

```sh
$ kubectl get pods -o wide
```

4. Get logs from a given pod

```sh
$ kubectl logs <pod-id, e.g. tg-dht-c95b5>
```

5. Check on the monitoring infrastructure (it runs in the monitoring namespace)

```sh
$ kubectl get pods --namespace monitoring
```

6. Get access to the Redis shell

```sh
$ kubectl port-forward svc/testground-infra-redis-master 6379:6379 &
$ redis-cli -h localhost -p 6379
```

## Use a Kubernetes context for another cluster

`kops` lets you download the entire Kubernetes context config.

If you want to let other people on your team connect to your Kubernetes cluster, you need to give them the information.

```sh
$ kops export kubecfg --state $KOPS_STATE_STORE --name=$NAME
```

## Known issues and future improvements

- [ ] Alerts (and maybe auto-scaling down) for idle clusters, so that we don't incur costs.
