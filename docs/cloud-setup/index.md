Testground Cloud Setup
======================

Here are some preliminary instructions for getting started setting up a cloud infrastructure for running Testground.

At Protocol Labs, initially, we are using AWS, but it should be possible to run it in any cloud provider with appropriate adjustments.

For "Phase Zero", we are making the assumption that Testground will be running in it's own isolated AWS account, and there will be limited effort into setting up fine-grained roles and permissions. Essentially, it will be an "admin party" ... the initial set of users will be fully trusted and granted full AWS admin rights.

In future phases, we'd like to evolve the system to support more access control with authorization and capabilities.

# Videos

Due to the nature of the setup involving a lot of interaction with web UI dashboards, we're going to document the setup using short screencast videos with supplemental notes.

# Core AWS Setup

This screencast shows how to log in to AWS with root credentials (using 1Password), create an IAM user account, and setup multi-factor auth.

# ElasticSearch / Kibana Setup

This screencast shows how to access the dashboard for the Elasticsearch Service (setup via the AWS Marketplace), provision a test cluster, and setup a user and a space in Kibana.

# Individual VM Setup

This screencast shows how to manually create an EC2 virtual machine and install dependencies suitable for running testground. 

# Running a Testground test

This screencast shows how to run a testground test and view the results in Kibana.