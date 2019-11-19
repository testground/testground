Terraform
---------

```
terraform apply \
  -var key_name={your_aws_key_name} \
  -var tag={value_for_TG_tag} \
  -var aws_instance_type_manager=c5.large \
  -var aws_instance_type_redis=c5.large \
  -var aws_instance_type_worker=m5.xlarge \
  -var workers=2
```

You can also create a `terraform.tfvars` files
and place it in the current directory, eg.

```
key_name = "jim-us-west-2"
tag      = "jim_dev"
```

Then you can just run `terraform apply`.

(use `terraform fmt` to make it pretty)


