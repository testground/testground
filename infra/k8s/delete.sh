#!/bin/bash

set -o errexit
set -o pipefail

set -e

err_report() {
    echo "Error on line $1"
}

trap 'err_report $LINENO' ERR

vpcId=`aws ec2 describe-vpcs --filters Name=tag:Name,Values=$NAME --output text | awk '/VPCS/ { print $8 }'`

if [[ -z ${vpcId} ]]; then
  echo "Couldn't detect AWS VPC created by `kops`"
  exit 1
fi

echo "Detected VPC: $vpcId"

securityGroupId=`aws ec2 describe-security-groups --output text | awk '/nodes.'$NAME'/ && /SECURITYGROUPS/ { print $6 };'`

if [[ -z ${securityGroupId} ]]; then
  echo "Couldn't detect AWS Security Group created by `kops`"
  exit 1
fi

echo "Detected Security Group ID: $securityGroupId"

subnetId=`aws ec2 describe-subnets --output text | awk '/'$vpcId'/ { print $12 }'`

if [[ -z ${subnetId} ]]; then
  echo "Couldn't detect AWS Subnet created by `kops`"
  exit 1
fi

echo "Detected Subnet ID: $subnetId"

pushd efs-terraform

# extract s3 bucket from kops state store
S3_BUCKET="${KOPS_STATE_STORE:5:100}"

terraform init -backend-config=bucket=$S3_BUCKET \
               -backend-config=key=tf-efs-$NAME \
               -backend-config=region=$AWS_REGION

terraform destroy -var aws_region=$AWS_REGION -var fs_subnet_id=$subnetId -var fs_sg_id=$securityGroupId -auto-approve

popd

kops delete cluster $NAME --yes
