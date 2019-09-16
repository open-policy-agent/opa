---
title: Terraform
kind: tutorial
weight: 1
---

Terraform lets you describe the infrastructure you want and automatically creates, deletes, and modifies
your existing infrastructure to match. OPA makes it possible to write policies that test the changes
Terraform is about to make before it makes them. Such tests help in different ways:

* tests help individual developers sanity check their Terraform changes
* tests can auto-approve run-of-the-mill infrastructure changes and reduce the burden of peer-review
* tests can help catch problems that arise when applying Terraform to production after applying it to staging


## Goals

In this tutorial, you'll learn how to use OPA to implement unit tests for Terraform plans that create
and delete auto-scaling groups and servers.

## Prerequisites

This tutorial requires

* [Terraform 0.12.6](https://releases.hashicorp.com/terraform/0.12.6/)
* [OPA](https://github.com/open-policy-agent/opa/releases)

(This tutorial *should* also work with the [latest version of Terraform](https://www.terraform.io/downloads.html), but 
it is untested.  Contributions welcome!)

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

Then initialize Terraform and ask it to calculate what changes it will make and store the output in `plan.binary`.

```shell
terraform init
terraform plan --out tfplan.binary
```

### 2. Convert the Terraform plan into JSON

Use the command [terraform show](https://www.terraform.io/docs/commands/show.html) to convert the Terraform plan into 
JSON so that OPA can read the plan.

```shell
terraform show -json tfplan.binary > tfplan.json
```

Here is the expected contents of `tfplan.json`.

```live:terraform:input
{
  "format_version": "0.1",
  "terraform_version": "0.12.6",
  "planned_values": {
    "root_module": {
      "resources": [
        {
          "address": "aws_autoscaling_group.my_asg",
          "mode": "managed",
          "type": "aws_autoscaling_group",
          "name": "my_asg",
          "provider_name": "aws",
          "schema_version": 0,
          "values": {
            "availability_zones": [
              "us-west-1a"
            ],
            "desired_capacity": 4,
            "enabled_metrics": null,
            "force_delete": true,
            "health_check_grace_period": 300,
            "health_check_type": "ELB",
            "initial_lifecycle_hook": [],
            "launch_configuration": "my_web_config",
            "launch_template": [],
            "max_size": 5,
            "metrics_granularity": "1Minute",
            "min_elb_capacity": null,
            "min_size": 1,
            "mixed_instances_policy": [],
            "name": "my_asg",
            "name_prefix": null,
            "placement_group": null,
            "protect_from_scale_in": false,
            "suspended_processes": null,
            "tag": [],
            "tags": null,
            "termination_policies": null,
            "timeouts": null,
            "wait_for_capacity_timeout": "10m",
            "wait_for_elb_capacity": null
          }
        },
        {
          "address": "aws_instance.web",
          "mode": "managed",
          "type": "aws_instance",
          "name": "web",
          "provider_name": "aws",
          "schema_version": 1,
          "values": {
            "ami": "ami-09b4b74c",
            "credit_specification": [],
            "disable_api_termination": null,
            "ebs_optimized": null,
            "get_password_data": false,
            "iam_instance_profile": null,
            "instance_initiated_shutdown_behavior": null,
            "instance_type": "t2.micro",
            "monitoring": null,
            "source_dest_check": true,
            "tags": null,
            "timeouts": null,
            "user_data": null,
            "user_data_base64": null
          }
        },
        {
          "address": "aws_launch_configuration.my_web_config",
          "mode": "managed",
          "type": "aws_launch_configuration",
          "name": "my_web_config",
          "provider_name": "aws",
          "schema_version": 0,
          "values": {
            "associate_public_ip_address": false,
            "enable_monitoring": true,
            "ephemeral_block_device": [],
            "iam_instance_profile": null,
            "image_id": "ami-09b4b74c",
            "instance_type": "t2.micro",
            "name": "my_web_config",
            "name_prefix": null,
            "placement_tenancy": null,
            "security_groups": null,
            "spot_price": null,
            "user_data": null,
            "user_data_base64": null,
            "vpc_classic_link_id": null,
            "vpc_classic_link_security_groups": null
          }
        }
      ]
    }
  },
  "resource_changes": [
    {
      "address": "aws_autoscaling_group.my_asg",
      "mode": "managed",
      "type": "aws_autoscaling_group",
      "name": "my_asg",
      "provider_name": "aws",
      "change": {
        "actions": [
          "create"
        ],
        "before": null,
        "after": {
          "availability_zones": [
            "us-west-1a"
          ],
          "desired_capacity": 4,
          "enabled_metrics": null,
          "force_delete": true,
          "health_check_grace_period": 300,
          "health_check_type": "ELB",
          "initial_lifecycle_hook": [],
          "launch_configuration": "my_web_config",
          "launch_template": [],
          "max_size": 5,
          "metrics_granularity": "1Minute",
          "min_elb_capacity": null,
          "min_size": 1,
          "mixed_instances_policy": [],
          "name": "my_asg",
          "name_prefix": null,
          "placement_group": null,
          "protect_from_scale_in": false,
          "suspended_processes": null,
          "tag": [],
          "tags": null,
          "termination_policies": null,
          "timeouts": null,
          "wait_for_capacity_timeout": "10m",
          "wait_for_elb_capacity": null
        },
        "after_unknown": {
          "arn": true,
          "availability_zones": [
            false
          ],
          "default_cooldown": true,
          "id": true,
          "initial_lifecycle_hook": [],
          "launch_template": [],
          "load_balancers": true,
          "mixed_instances_policy": [],
          "service_linked_role_arn": true,
          "tag": [],
          "target_group_arns": true,
          "vpc_zone_identifier": true
        }
      }
    },
    {
      "address": "aws_instance.web",
      "mode": "managed",
      "type": "aws_instance",
      "name": "web",
      "provider_name": "aws",
      "change": {
        "actions": [
          "create"
        ],
        "before": null,
        "after": {
          "ami": "ami-09b4b74c",
          "credit_specification": [],
          "disable_api_termination": null,
          "ebs_optimized": null,
          "get_password_data": false,
          "iam_instance_profile": null,
          "instance_initiated_shutdown_behavior": null,
          "instance_type": "t2.micro",
          "monitoring": null,
          "source_dest_check": true,
          "tags": null,
          "timeouts": null,
          "user_data": null,
          "user_data_base64": null
        },
        "after_unknown": {
          "arn": true,
          "associate_public_ip_address": true,
          "availability_zone": true,
          "cpu_core_count": true,
          "cpu_threads_per_core": true,
          "credit_specification": [],
          "ebs_block_device": true,
          "ephemeral_block_device": true,
          "host_id": true,
          "id": true,
          "instance_state": true,
          "ipv6_address_count": true,
          "ipv6_addresses": true,
          "key_name": true,
          "network_interface": true,
          "network_interface_id": true,
          "password_data": true,
          "placement_group": true,
          "primary_network_interface_id": true,
          "private_dns": true,
          "private_ip": true,
          "public_dns": true,
          "public_ip": true,
          "root_block_device": true,
          "security_groups": true,
          "subnet_id": true,
          "tenancy": true,
          "volume_tags": true,
          "vpc_security_group_ids": true
        }
      }
    },
    {
      "address": "aws_launch_configuration.my_web_config",
      "mode": "managed",
      "type": "aws_launch_configuration",
      "name": "my_web_config",
      "provider_name": "aws",
      "change": {
        "actions": [
          "create"
        ],
        "before": null,
        "after": {
          "associate_public_ip_address": false,
          "enable_monitoring": true,
          "ephemeral_block_device": [],
          "iam_instance_profile": null,
          "image_id": "ami-09b4b74c",
          "instance_type": "t2.micro",
          "name": "my_web_config",
          "name_prefix": null,
          "placement_tenancy": null,
          "security_groups": null,
          "spot_price": null,
          "user_data": null,
          "user_data_base64": null,
          "vpc_classic_link_id": null,
          "vpc_classic_link_security_groups": null
        },
        "after_unknown": {
          "ebs_block_device": true,
          "ebs_optimized": true,
          "ephemeral_block_device": [],
          "id": true,
          "key_name": true,
          "root_block_device": true
        }
      }
    }
  ],
  "configuration": {
    "provider_config": {
      "aws": {
        "name": "aws",
        "expressions": {
          "region": {
            "constant_value": "us-west-1"
          }
        }
      }
    },
    "root_module": {
      "resources": [
        {
          "address": "aws_autoscaling_group.my_asg",
          "mode": "managed",
          "type": "aws_autoscaling_group",
          "name": "my_asg",
          "provider_config_key": "aws",
          "expressions": {
            "availability_zones": {
              "constant_value": [
                "us-west-1a"
              ]
            },
            "desired_capacity": {
              "constant_value": 4
            },
            "force_delete": {
              "constant_value": true
            },
            "health_check_grace_period": {
              "constant_value": 300
            },
            "health_check_type": {
              "constant_value": "ELB"
            },
            "launch_configuration": {
              "constant_value": "my_web_config"
            },
            "max_size": {
              "constant_value": 5
            },
            "min_size": {
              "constant_value": 1
            },
            "name": {
              "constant_value": "my_asg"
            }
          },
          "schema_version": 0
        },
        {
          "address": "aws_instance.web",
          "mode": "managed",
          "type": "aws_instance",
          "name": "web",
          "provider_config_key": "aws",
          "expressions": {
            "ami": {
              "constant_value": "ami-09b4b74c"
            },
            "instance_type": {
              "constant_value": "t2.micro"
            }
          },
          "schema_version": 1
        },
        {
          "address": "aws_launch_configuration.my_web_config",
          "mode": "managed",
          "type": "aws_launch_configuration",
          "name": "my_web_config",
          "provider_config_key": "aws",
          "expressions": {
            "image_id": {
              "constant_value": "ami-09b4b74c"
            },
            "instance_type": {
              "constant_value": "t2.micro"
            },
            "name": {
              "constant_value": "my_web_config"
            }
          },
          "schema_version": 0
        }
      ]
    }
  }
}
```

The json plan output produced by terraform contains a lot of information. For this tutorial, we will be interested by:

* `.resource_changes`: array containing all the actions that terraform will apply on the infrastructure.
* `.resource_changes[].type`: the type of resource (eg `aws_instance` , `aws_iam` ...)
* `.resource_changes[].change.actions`: array of actions applied on the resource (`create`, `update`, `delete`...)

For more information about the json plan representation, please check the [terraform documentation](https://www.terraform.io/docs/internals/json-format.html#plan-representation)

### 3. Write the OPA policy to check the plan

The policy computes a score for a Terraform that combines

* The number of deletions of each resource type
* The number of creations of each resource type
* The number of modifications of each resource type

The policy authorizes the plan when the score for the plan is below a threshold
and there are no changes made to any IAM resources.
(For simplicity, the threshold in this tutorial is the same for everyone, but in
practice you would vary the threshold depending on the user.)

**terraform.rego**:

```live:terraform:module:openable
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
    all := [ x |
            some resource_type
            crud := weights[resource_type];
            del := crud["delete"] * num_deletes[resource_type];
            new := crud["create"] * num_creates[resource_type];
            mod := crud["modify"] * num_modifies[resource_type];
            x := del + new + mod
    ]
    s := sum(all)
}

# Whether there is any change to IAM
touches_iam {
    all := resources["aws_iam"]
    count(all) > 0
}

####################
# Terraform Library
####################

# list of all resources of a given type
resources[resource_type] = all {
    some resource_type
    resource_types[resource_type]
    all := [name |
        name:= tfplan.resource_changes[_]
        name.type == resource_type
    ]
}

# number of creations of resources of a given type
num_creates[resource_type] = num {
    some resource_type
    resource_types[resource_type]
    all := resources[resource_type]
    creates := [res |  res:= all[_]; res.change.actions[_] == "create"]
    num := count(creates)
}


# number of deletions of resources of a given type
num_deletes[resource_type] = num {
    some resource_type
    resource_types[resource_type]
    all := resources[resource_type]
    deletions := [res |  res:= all[_]; res.change.actions[_] == "delete"]
    num := count(deletions)
}

# number of modifications to resources of a given type
num_modifies[resource_type] = num {
    some resource_type
    resource_types[resource_type]
    all := resources[resource_type]
    modifies := [res |  res:= all[_]; res.change.actions[_] == "update"]
    num := count(modifies)
}
```

### 4. Evaluate the OPA policy on the Terraform plan

To evaluate the policy against that plan, you hand OPA the policy, the Terraform plan as input, and
ask it to evaluate `data.terraform.analysis.authz`.

```shell
opa eval --format pretty --data terraform.rego --input tfplan.json "data.terraform.analysis.authz"
```
```live:terraform/authz:query:hidden
data.terraform.analysis.authz
```
```live:terraform/authz:output
```


If you're curious, you can ask for the score that the policy used to make the authorization decision.
In our example, it is 11 (10 for the creation of the auto-scaling group and 1 for the creation of the server).

```shell
opa eval --format pretty --data terraform.rego --input tfplan.json "data.terraform.analysis.score"
```

```live:terraform/score:query:hidden
data.terraform.analysis.score
```
```live:terraform/score:output
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
terraform init
terraform plan --out tfplan_large.binary
terraform show -json tfplan_large.binary > tfplan_large.json
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
