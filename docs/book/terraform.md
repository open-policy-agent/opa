# Terraform Testing

Terraform lets you describe the infrastructure you want and automatically creates, deletes, and modifies
your existing infrastructure to match.  OPA makes it possible to write policies that test the changes
Terraform is about to make before it makes them.  Such tests help in different ways:
* tests help individual developers sanity check their Terraform changes
* tests can auto-approve run-of-the-mill infrastructure changes and reduce
the burden of peer-review
* tests can help catch problems that arise when applying Terraform to production after applying it to staging


## Goals

In this tutorial, you'll learn how to use OPA to implement unit tests for Terraform plans that create
and delete auto-scaling groups and servers.

## Prerequisites

This tutorial requires

* [Terraform 0.8](https://releases.hashicorp.com/terraform/0.8.8/)
* [OPA](https://github.com/open-policy-agent/opa/releases)
* [tfjson](https://github.com/palantir/tfjson) (`go get github.com/palantir/tfjson`): a Go utility that converts Terraform plans into JSON

(This tutorial *should* also work with the [latest version of Terraform](https://www.terraform.io/downloads.html) 
and the [latest version of tfjson](https://github.com/philips/tfjson), but it is untested.  Contributions welcome!)

## Steps

### 1. Create and save a Terraform plan

Create a [Terraform](https://www.terraform.io/docs/index.html) file that includes an 
auto-scaling group and a server on AWS.  (You will need to modify the `shared_credentials_file`
to point to your AWS credentials.)

```shell
cat >main.tf <<EOF
provider "aws" {
    region = "us-west-1"
}
resource "aws_instance" "web" {
  instance_type = "t2.micro"
  ami = "ami-09b4b74c"
}
resource "aws_autoscaling_group" "my_asg" {
  availability_zones        = ["us-west-1a"]
  name                      = "my_asg"
  max_size                  = 5
  min_size                  = 1
  health_check_grace_period = 300
  health_check_type         = "ELB"
  desired_capacity          = 4
  force_delete              = true
  launch_configuration      = "my_web_config"
}
resource "aws_launch_configuration" "my_web_config" {
    name = "my_web_config"
    image_id = "ami-09b4b74c"
    instance_type = "t2.micro"
}
EOF
```

Then ask Terraform to calculate what changes it will make and store the output in `plan.binary`.

```shell
terraform plan --out tfplan.binary
```

### 2. Convert the Terraform plan into JSON

Use the `tfjson` tool to convert the Terraform plan into JSON so that OPA can read the plan.

```shell
tfjson tfplan.binary > tfplan.json
```

Here is the expected contents of `tfplan.json`.
```
{
    "aws_autoscaling_group.my_asg": {
        "arn": "",
        "availability_zones.#": "1",
        "availability_zones.3205754986": "us-west-1a",
        "default_cooldown": "",
        "desired_capacity": "4",
        "destroy": false,
        "destroy_tainted": false,
        "force_delete": "true",
        "health_check_grace_period": "300",
        "health_check_type": "ELB",
        "id": "",
        "launch_configuration": "my_web_config",
        "load_balancers.#": "",
        "max_size": "5",
        "metrics_granularity": "1Minute",
        "min_size": "1",
        "name": "my_asg",
        "protect_from_scale_in": "false",
        "vpc_zone_identifier.#": "",
        "wait_for_capacity_timeout": "10m"
    },
    "aws_instance.web": {
        "ami": "ami-09b4b74c",
        "associate_public_ip_address": "",
        "availability_zone": "",
        "destroy": false,
        "destroy_tainted": false,
        "ebs_block_device.#": "",
        "ephemeral_block_device.#": "",
        "id": "",
        "instance_state": "",
        "instance_type": "t2.micro",
        "ipv6_addresses.#": "",
        "key_name": "",
        "network_interface_id": "",
        "placement_group": "",
        "private_dns": "",
        "private_ip": "",
        "public_dns": "",
        "public_ip": "",
        "root_block_device.#": "",
        "security_groups.#": "",
        "source_dest_check": "true",
        "subnet_id": "",
        "tenancy": "",
        "vpc_security_group_ids.#": ""
    },
    "aws_launch_configuration.my_web_config": {
        "associate_public_ip_address": "false",
        "destroy": false,
        "destroy_tainted": false,
        "ebs_block_device.#": "",
        "ebs_optimized": "",
        "enable_monitoring": "true",
        "id": "",
        "image_id": "ami-09b4b74c",
        "instance_type": "t2.micro",
        "key_name": "",
        "name": "my_web_config",
        "root_block_device.#": ""
    },
    "destroy": false
}
```

### 3. Write the OPA policy to check the plan

The policy computes a score for a Terraform that combines
* The number of deletions of each resource type
* The number of creations of each resource type
* The number of modifications of each resource type

The policy authorizes the plan when the score for the plan is below a threshold
and there are no changes made to any IAM resources.
(For simplicity, the threshold in this tutorial is the same for everyone, but in
practice you would vary the threshold depending on the user.)

```shell
cat >terraform.rego <<EOF
package terraform.analysis

import input as tfplan

########################
# Parameters for Policy
########################

# acceptable score for automated authorization
blast_radius = 30

# weights assigned for each operation on each resource-type
weights = {
    "aws_autoscaling_group": {"delete": 100, "create": 10, "modify": 1},
    "aws_instance": {"delete": 10, "create": 1, "modify": 1}
}

# Consider exactly these resource types in calculations
resource_types = {"aws_autoscaling_group", "aws_instance", "aws_iam", "aws_launch_configuration"}

#########
# Policy
#########

# Authorization holds if score for the plan is acceptable and no changes are made to IAM
default authz = false
authz {
    score < blast_radius
    not touches_iam
}

# Compute the score for a Terraform plan as the weighted sum of deletions, creations, modifications
score = s {
    all = [ x | 
            weights[resource_type] = crud;
            del = crud["delete"] * num_deletes[resource_type];
            new = crud["create"] * num_creates[resource_type];
            mod = crud["modify"] * num_modifies[resource_type];
            x1 = del + new
            x = x1 + mod
    ]
    sum(all, s)
}

# Whether there is any change to IAM
touches_iam {
    all = instance_names["aws_iam"]
    count(all, c)
    c > 0
}

####################
# Terraform Library
####################

# list of all resources of a given type
instance_names[resource_type] = all {
    resource_types[resource_type]
    all = [name |
        tfplan[name] = _
        startswith(name, resource_type)
    ]
}

# number of deletions of resources of a given type
num_deletes[resource_type] = num {
    resource_types[resource_type]
    all = instance_names[resource_type]
    deletions = [name | all[_] = name; tfplan[name]["destroy"] = true]
    count(deletions, num)
}

# number of creations of resources of a given type
num_creates[resource_type] = num {
    resource_types[resource_type]
    all = instance_names[resource_type]
    creates = [name | all[_] = name; tfplan[name]["id"] = ""]
    count(creates, num)
}

# number of modifications to resources of a given type
num_modifies[resource_type] = num {
    resource_types[resource_type]
    all = instance_names[resource_type]
    modifies = [name | all[_] = name; obj = tfplan[name]; obj["destroy"] = false; not obj["id"]]
    count(modifies, num)
}
EOF
```

### 4. Evaluate the OPA policy on the Terraform plan

To evaluate the policy against that plan, you hand OPA the policy, the Terraform plan as input, and
ask it to evaluate `data.terraform.analysis.authz`.

```shell
opa eval --data terraform.rego --input tfplan.json "data.terraform.analysis.authz"
```

If you're curious, you can ask for the score that the policy used to make the authorization decision.
In our example, it is 11 (10 for the creation of the auto-scaling group and 1 for the creation of the server).

```shell
opa eval --data terraform.rego --input tfplan.json "data.terraform.analysis.score"
```

If as suggested in the previous step, you want to modify your policy to make an authorization decision
based on both the user and the Terraform plan, the input you would give to OPA would take the form
`{"user": <user>, "plan": <plan>}`, and your policy would reference the user with `input.user` and
the plan with `input.plan`.  You could even go so far as to provide the Terraform state file and the AWS
EC2 data to OPA and write policy using all of that context.

### 5. Create a Large Terraform plan and Evaluate it

Create a Terraform plan that creates enough resources to exceed the blast-radius permitted
by policy.  

```shell
cat >main.tf <<EOF
provider "aws" {
    region = "us-west-1"
}
resource "aws_instance" "web" {
  instance_type = "t2.micro"
  ami = "ami-09b4b74c"
}
resource "aws_autoscaling_group" "my_asg" {
  availability_zones        = ["us-west-1a"]
  name                      = "my_asg"
  max_size                  = 5
  min_size                  = 1
  health_check_grace_period = 300
  health_check_type         = "ELB"
  desired_capacity          = 4
  force_delete              = true
  launch_configuration      = "my_web_config"
}
resource "aws_launch_configuration" "my_web_config" {
    name = "my_web_config"
    image_id = "ami-09b4b74c"
    instance_type = "t2.micro"
}
resource "aws_autoscaling_group" "my_asg2" {
  availability_zones        = ["us-west-2a"]
  name                      = "my_asg2"
  max_size                  = 6
  min_size                  = 1
  health_check_grace_period = 300
  health_check_type         = "ELB"
  desired_capacity          = 4
  force_delete              = true
  launch_configuration      = "my_web_config"
}
resource "aws_autoscaling_group" "my_asg3" {
  availability_zones        = ["us-west-2b"]
  name                      = "my_asg3"
  max_size                  = 7
  min_size                  = 1
  health_check_grace_period = 300
  health_check_type         = "ELB"
  desired_capacity          = 4
  force_delete              = true
  launch_configuration      = "my_web_config"
}
EOF
```

Generate the Terraform plan and convert it to JSON.

```shell
terraform plan --out tfplan_large.binary
tfjson tfplan_large.binary > tfplan_large.json
```

Evaluate the policy to see that it fails the policy tests and check the score.

```shell
opa eval --data terraform.rego --input tfplan_large.json "data.terraform.analysis.authz"
opa eval --data terraform.rego --input tfplan_large.json "data.terraform.analysis.score"
```


### 6. (Optional) Run OPA as a daemon and evaluate policy 

In addition to running OPA from the command-line, you can run it as a daemon loaded with the Terraform policy and
then interact with it using its HTTP API.  First, start the daemon:

```shell
opa run -s terraform.rego
```

Then in a separate terminal, use OPA's HTTP API to evaluate the policy against the two Terraform plans.

```shell
curl localhost:8181/v0/data/terraform/analysis/authz -d @tfplan.json
curl localhost:8181/v0/data/terraform/analysis/authz -d @tfplan_large.json
```


## Wrap Up

Congratulations for finishing the tutorial!

You learned a number of things about Terraform Testing with OPA:

* OPA gives you fine-grained policy control over Terraform plans.
* You can use data other than the plan itself (e.g. the user) when writing authorization policies.

Keep in mind that it's up to you to decide how to use OPA's Terraform tests and authorization decision.  Here are some ideas.
* Add it as part of your Terraform wrapper to implement unit tests on Terraform plans
* Use it to automatically approve run-of-the-mill Terraform changes to reduce the burden of peer-review
* Embed it into your deployment system to catch problems that arise when applying Terraform to production after applying it to staging
