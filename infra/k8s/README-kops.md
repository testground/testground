# Setting up a self-managed Kubernetes cluster on AWS for Testground

In this directory, you will find:

```
» tree
.
├── README-kops.md
└── kops                   # Kubernetes resources for setting up networking with Flannel
```

## Requirements

- 1. [kops](https://github.com/kubernetes/kops/releases). >= 1.17.0-alpha.1

## Set up infrastructure with Terraform

1. [Configure your AWS credentials](https://docs.aws.amazon.com/cli/)

2. Create a bucket for KOPS state. This is similar to Terraform state bucket.

```
aws s3api create-bucket \
    --bucket kops-backend-bucket \
    --region eu-central-1 --create-bucket-configuration LocationConstraint=eu-central-1
```

3. Pick up a cluster name, and set zone and KOPS state store

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

6. Install Flannel and Multus

```
kubectl apply -f ./flannel.yml
kubectl apply -f ./multus.yml
```

7. Get the `master node name` and `master ip address` from `kubectl`

```
MASTER_NAME=`kubectl get nodes -o wide | grep master | awk '{print $1}'`
MASTER_IP=`kubectl get nodes -o wide | grep master | awk '{print $6}'`

echo $MASTER_NAME
echo $MASTER_IP
```

8. Update configuration for secondary overlay network with Flannel

```
sed 's/__MASTER_NAME__/'"$MASTER_NAME"'/g' flannel2.yml-example > tmp
sed 's/__MASTER_IP__/'"$MASTER_IP"'/g' tmp > flannel2.yml
rm tmp
```

9. Create the secondary overlay network, and run the Flannel daemon

```
kubectl apply -f ./flannel2.yml
```

10. Create `NetworkAttachmentDefinition` for both networks (although the first is actually the default, and probably redundant)

```
kubectl apply -f ./flannel-conf.yml
kubectl apply -f ./flannel2-conf.yml
```

11. Create a sample pod and attach it to both networks

```
kubectl apply -f sample-pod.yml
kubectl apply -f sample-pod2.yml
```
