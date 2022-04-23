---
title: AWS CloudFormation Hooks
kind: tutorial
weight: 1
---

[AWS CloudFormation Hooks](https://docs.aws.amazon.com/cloudformation-cli/latest/userguide/hooks.html) allows users to
verify AWS infrastructure components defined in AWS CloudFormation
[templates](https://aws.amazon.com/cloudformation/resources/templates/), like S3 Buckets or EC2 instances, prior to
deployment. This is done via **hooks**. Hooks are composed of custom code running in an AWS Lambda function, which is
invoked before a resource is created, updated or deleted.

AWS currently supports hooks written in either Java or Python, and provides a
[sample repository](https://github.com/aws-cloudformation/aws-cloudformation-samples), which includes example hooks
written in both languages. Since we'd rather use OPA for this purpose, we'd need some code to process the requests
handled by the hook and send them forward to OPA for policy decisions via its
[REST API](https://www.openpolicyagent.org/docs/latest/rest-api/) using
the [OPA AWS CloudFormation Hook](https://github.com/StyraInc/opa-aws-cloudformation-hook).

## Goals

This tutorial shows how to deploy an AWS CloudFormation Hook that forwards requests to OPA for policy decisions,
allowing us to use policy to determine whether a request to create, update or delete a resource should be
allowed or denied. We'll learn how to author policies that take the input structure of CloudFormation Templates into
account, and some special considerations to be aware of in this environment.

In addition, this tutorial shows how we can leverage dynamic policy composition to group and structure our policies in a
way that follows the domain to which they apply.

## Prerequisites

In order to complete this tutorial, the following prerequisites needs to be met:

* An AWS account, with permissions to deploy resources via AWS CloudFormation, and valid credentials available to the CLI commands
* The [AWS CLI](https://aws.amazon.com/cli/) (`aws`) tool
* The [CloudFormation CLI](https://docs.aws.amazon.com/cloudformation-cli/latest/userguide/what-is-cloudformation-cli.html) (`cfn`) tool
* Docker
* OPA server running at an endpoint reachable by the AWS Lambda function, either within the same AWS environment, or
  elsewhere. While developing your CloudFormation policies, a good option is to run OPA locally, but exposed to the
  public via a service like [ngrok](https://ngrok.com/).

## Steps

### 1. Install the CloudFormation Hook

To start out, clone the OPA AWS CloudFormation Hook repository:

```shell
git clone https://github.com/StyraInc/opa-aws-cloudformation-hook.git
cd opa-aws-cloudformation-hook
```

To install (but not activate) the hook provided in this repository into your AWS account, cd into the `hooks` directory
and run:

```shell
cd hooks
cfn submit --set-default
```

When the command above is finished (this may take several minutes), you should see output similar to this:

```
Successfully submitted type. Waiting for registration with token '16697881-de36-45b8-8bc4-d9744431fa82' to complete.
Registration complete.
{
  'ProgressStatus': 'COMPLETE',
  'Description': 'Deployment is currently in DEPLOY_STAGE of status COMPLETED',
  'TypeArn': 'arn:aws:cloudformation:eu-north-1:687803501377:type/hook/Styra-OPA-Hook',
  ...
}
```

### 2. Configure the OPA AWS CloudFormation Hook

The hook is now installed but needs to be configured for your environment. First, copy the value of the `TypeArn`
attribute from the JSON output of the above command, and store it in an environment variable:

```shell
export HOOK_TYPE_ARN="arn:aws:cloudformation:eu-north-1:687803501377:type/hook/Styra-OPA-Hook"
```

Next, set the AWS region and the URL to use for calling OPA:

```shell
export AWS_REGION="eu-north-1"
export OPA_URL="https://cfn-opa.example.com"
```

**(OPTIONAL):** If you want to use a bearer token to authenticate against OPA, provide an ARN pointing to the AWS Secret
containing the token:

```shell
export OPA_AUTH_TOKEN_SECRET="arn:aws:secretsmanager:eu-north-1:687803501377:secret:opa-cfn-token-l26bHK"
```

With the configuration variables set, push the configuration to AWS (remove `opaAuthTokenSecret` if you don't intend to
use it):

```shell
aws cloudformation --region "$AWS_REGION" set-type-configuration \
  --configuration "{\"CloudFormationConfiguration\":{\"HookConfiguration\":{\"TargetStacks\":\"ALL\",\"FailureMode\":\"FAIL\",\"Properties\":{\"opaUrl\": \"$OPA_URL\",\"opaAuthTokenSecret\":\"$OPA_AUTH_TOKEN_SECRET\"}}}}" \
  --type-arn $HOOK_TYPE_ARN
```

The hook is now installed, configured and activated!

### 3. Learn the Domain

Before we proceed to write our first policy, let's take a closer look at the data we'll be working with.

#### AWS CloudFormation Templates

A template file is commonly a YAML or JSON file, describing a set of AWS resources. While a template may describe
multiple resources, the hook will send each resource for validation separately. Important to note here is that the
resource presented to the hook will be shown **exactly** as provided in the template file. The hook does not perform any
type preprocessing, such as adding default values where missing, or providing auto-generated names. Policy authors must
hence take into account that even "obvious" attributes like name might not be present in the resource provided for
evaluation. As an example, a template to deploy an S3 Bucket with default attributes may be as minimal as this:

```yaml
Resources:
  ExampleS3Bucket:
    Type: AWS::S3::Bucket
```

For more information on templates, see the
[AWS User Guide](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/template-guide.html) on that topic.

#### Input and Response Format

The OPA configured to receive requests from the CFN hook will have its input provided in this format:

```json
{
  "action": "CREATE",
  "hook": "Styra::OPA::Hook",
  "resource": {
    "id": "MyS3Bucket",
    "name": "AWS::S3::Bucket",
    "type": "AWS::S3::Bucket",
    "properties": {
      "Tags": [{"Key": "Owner", "Value": "Platform Team"}],
      "BucketName": "platform-bucket-1"
    }
  }
}
```

* The `action` is either `CREATE`, `UPDATE` or `DELETE`
* The `id` is the key of the resource, as provided in the template
* The `type` is divided by "resource domain" and the specific type, so e.g. the S3 domain may contain `Bucket`,
  `BucketPolicy`, and so on.

The hook expects the response to contain a boolean `allow` attribute, and a list of (potential) `violations`:

```json
{
  "allow": false,
  "violations": ["bucket must not be public", "bucket name must follow naming standard"]
}
```

Any request denied will be logged in [AWS CloudWatch](https://aws.amazon.com/cloudwatch/) for the same account.

### 4. Write a CloudFormation Hook Policy

With knowledge of the domain and the data model, we're ready to write our first CloudFormation Hook policy. Since we'll
have a single OPA endpoint servicing requests for all types of resources, we'll use the
[default decision](../configuration/#miscellaneous) policy, which by default queries the `system.main` rule. Let's add a
simple policy to block an S3 Bucket unless it has an `AccessControl` attribute set to `Private`:

```live:example/system:module
package system

import future.keywords

main := {
    "allow": count(deny) == 0,
    "violations": deny,
}

deny[msg] {
    bucket_create_or_update
    not bucket_is_private

    msg := sprintf("S3 Bucket %s 'AccessControl' attribute value must be 'Private'", [input.resource.id])
}

bucket_create_or_update {
    input.resource.type == "AWS::S3::Bucket"
    input.action in {"CREATE", "UPDATE"}
}

bucket_is_private {
    input.resource.properties.AccessControl == "Private"
}
```

Since we know that CloudFormation Templates may contain only the bare minimum of information, we can't assume that there
will be an `AccessControl` attribute present in the input at all. Using negation of boolean rules inside of our `deny`
rules help alleviate the problem of values potentially being undefined. Compare to the following deny rule, which might
look correct at a first glance:

```live:example/fail:module
deny[msg] {
    bucket_create_or_update

    input.resource.properties.AccessControl != "Private"

    msg := sprintf("S3 Bucket %s 'AccessControl' attribute value must be 'Private'", [input.resource.id])
}
```

This rule would work fine as long as there _is_ an `AccessControl` attribute present in the input resource, but would
fail (i.e. not evaluate) as soon as the property was missing, leading to the resource being allowed! Using helper rules
and negation is a good way to work with data that might or might not be present, and results in more readable policies,
too.

{{< danger >}}
Surprisingly, boolean values from CloudFormation Templates are provided to the hook in the form of **strings** (i.e.
"true" and "false"). Policy authors must take this into account, and explicitly check for the value of these
attributes. An example S3 bucket policy might for example want to check that public ACLs are blocked:

```live:example/boolean_fail:module:read_only
# Wrong: will allow both "true" and "false" values as both are considered "truthy"
block_public_acls {
    input.resource.properties.PublicAccessBlockConfiguration.BlockPublicAcls
}
```

```live:example/boolean_correct:module:read_only
# Correct: will allow only when property set to "true"
block_public_acls {
    input.resource.properties.PublicAccessBlockConfiguration.BlockPublicAcls == "true"
}
```
{{< /danger >}}

### 5. Policy Enforcement Testing

With the above policy loaded into OPA, we may proceed to try it out. Let's deploy the minimal S3 Bucket from the
previous template example. Save the below minimal template to a file called `s3bucket.yaml`:

```yaml
Resources:
  ExampleS3Bucket:
    Type: AWS::S3::Bucket
```

Since our S3 bucket doesn't have an `AccessControl` attribute, it should be denied by the hook. We
deploy a template by creating a **stack**:

```shell
aws cloudformation create-stack --stack-name cfn-s3 --template-body file://s3bucket.yaml
```

The output of the above command will simply be a confirmation that the stack was deployed. It won't tell us whether the
deployment was successful or not. In order to know that, we'll need to check the stack events:

```shell
aws cloudformation describe-stack-events --stack-name cfn-s3
```

The output of the above command will be a list of all events associated with the `cfn-s3` stack. Among the events, you
should now find an item describing that the hook denied the request, and its reason for doing so:

```json
{
  "StackEvents": [
    {
      "StackId": "arn:aws:cloudformation:eu-north-1:55523455647:stack/cfn-s3/4f605f70-b1ca-12ec-b4d8-0a63e869dfee",
      "EventId": "ExampleS3Bucket-c243efd6-bfe7-3f10-8304-a0e40fe5f6f4",
      "StackName": "cfn-s3",
      "LogicalResourceId": "ExampleS3Bucket",
      "PhysicalResourceId": "",
      "ResourceType": "AWS::S3::Bucket",
      "Timestamp": "2022-03-31T08:41:15.946000+00:00",
      "ResourceStatus": "CREATE_IN_PROGRESS",
      "HookType": "Styra::OPA::Hook",
      "HookStatus": "HOOK_COMPLETE_FAILED",
      "HookStatusReason": "Hook failed with message: S3 Bucket ExampleS3Bucket 'AccessControl' attribute value must be 'Private'",
      "HookInvocationPoint": "PRE_PROVISION",
      "HookFailureMode": "FAIL"
    }
  ]
}
```

Congratulations! You've just successfully enforced your first CloudFormation Hook policy using OPA. Let's update the
template so that it passes our policy requirement:

**s3bucket.yaml**
```yaml
Resources:
  ExampleS3Bucket:
    Type: AWS::S3::Bucket
    Properties:
      AccessControl: Private
```

Even though our stack did not create an S3 bucket (as the change got rolled back), the **stack** still exists.
In order to try again, we'll first need to delete the existing stack:

```shell
aws cloudformation delete-stack --stack-name cfn-s3
```

Now, let's try again:

```shell
aws cloudformation create-stack --stack-name cfn-s3 --template-body file://s3bucket.yaml
```

Checking the output of `aws cloudformation describe-stack-events --stack-name cfn-s3` once more will now show that the
resource was created. Do note that this could take up to a minute, so if you don't see it immediately, rerun the command
a bit later.

```json
{
  "StackEvents": [
    {
      "StackId": "arn:aws:cloudformation:eu-north-1:55523455647:stack/cfn-s3/4f605f70-b1ca-12ec-b4d8-0a63e869dfee",
      "EventId": "e20fdfa0-b0d0-11ec-b669-0e70f1f560a6",
      "StackName": "cfn-s3",
      "LogicalResourceId": "cfn-s3",
      "PhysicalResourceId": "arn:aws:cloudformation:eu-north-1:55523455647:stack/cfn-s3/cf418141-b0d9-11bc-b421-0a1244c68dd1",
      "ResourceType": "AWS::CloudFormation::Stack",
      "Timestamp": "2022-03-31T08:59:33.392000+00:00",
      "ResourceStatus": "CREATE_COMPLETE"
    }
  ]
}
```

Note: once our stack is successfully deployed, we can use the `update-stack` command after we've made changes to our
templates:

```shell
aws cloudformation update-stack --stack-name cfn-s3 --template-body file://s3bucket.yaml`
```

## Further Improvements

### Dynamic Policy Composition

Having a single policy file for all rules will quickly become unwieldy. Could we improve this somehow? One way of doing
that would be to use dynamic policy composition, where a single main policy acts as a "router", and forwards queries to
other packages based on attributes from the input. A natural attribute to use for CloudFormation templates might for
example be the resource type, allowing us to group our policies by the resource type they're meant to act on. Let's
take a look at what such a main policy might look like:

**main.rego**
```live:example/router:module
# METADATA
# description: |
#   Dynamic routing to policy based in input.resource.type,
#   aggregating the deny rules found in all policies with a
#   matching package name
#
package system

import future.keywords

main := {
	"allow": count(violations) == 0,
	"violations": violations,
}

# METADATA
# description: |
#   Main routing logic, simply converting input.resource.type, e.g.
#   AWS::S3::Bucket to data.aws.s3.bucket and returning that document.
#
#   By default, only input.action == "CREATE" | "UPDATE" will be routed
#   to the data.aws.s3.bucket document. If handling "DELETE" actions is
#   desirable, one may create a special policy for that by simply appending
#   "delete" to the package name, e.g. data.aws.s3.bucket.delete
#
route := document(lower(component), lower(type)) {
	["AWS", component, type] = split(input.resource.type, "::")
}

violations[msg] {
	# Aggregate all deny rules found in routed document
	some msg in route.deny
}

#
# Basic input validation to avoid having to do this in each resource policy
#

violations["Missing input.resource"] {
	not input.resource
}

violations["Missing input.resource.type"] {
	not input.resource.type
}

violations["Missing input.resource.id"] {
	not input.resource.id
}

violations["Missing input.action"] {
	not input.action
}

#
# Helpers
#

document(component, type) := data.aws[component][type] {
	input.action != "DELETE"
}

document(component, type) := data.aws[component][type].delete {
	input.action == "DELETE"
}
```

The above policy will invoke the `route` rule to determine which package should be evaluated based on the
`input.resource.type`, transforming a value such as `AWS::S3::Bucket` into a call to the `data.aws.s3.bucket` package,
where each rule named `deny` will be evaluated, and the result aggregated into the final decision.

Since most of our policies will only deal with `CREATE` or `UPDATE` actions, we'd rather want to avoid having to check
for this in all of our rules. Instead, we'll have the router append `.delete` to the package name for `DELETE`
operations, so that a request to delete e.g. an S3 bucket would invoke the `data.aws.s3.bucket.delete` package (if it
exists).

Additionally, we'll also do some simple input validation at this stage, so that we may avoid doing so in our resource
specific policies.

We can now modify our original policy to verify S3 bucket resources only:

```live:example/bucket:module
package aws.s3.bucket

deny[sprintf("S3 Bucket %s 'AccessControl' attribute value must be 'Private'", [input.resource.id])] {
    not bucket_is_private
}

bucket_is_private {
    input.resource.properties.AccessControl == "Private"
}
```

Note how we no longer need the `bucket_create_or_update` rule, as that is already asserted by the main policy.
Quite an improvement in terms of readability, and a good foundation for further policy authoring. If you'd like to see
more examples of policy utilizing this pattern, check out the
[policy directory](https://github.com/StyraInc/opa-aws-cloudformation-hook/tree/main/policy) in the OPA AWS
CloudFormation Hook repo.

### OPA Authentication via AWS Secrets

#### OPA Configuration

Since the OPA server does not run inside the AWS Lambda, it is a good idea to require authentication to access its REST
API, as described in the OPA [documentation](https://www.openpolicyagent.org/docs/latest/security/#authentication-and-authorization).

A simple authz policy for checking the bearer token might look something like this:

**authz.rego**
```live:example/authz:module
package system.authz

default allow := false

allow {
    input.identity == "my_secret_token"
}
```

Once created, remember to pass the appropriate flags to `opa run` to enable authentication / authorization:

```shell
opa run --server --authentication=token --authorization=basic .
```

#### OPA AWS CloudFormation Hook Configuration

If configured to use a bearer token for authenticating against OPA (by setting the `OPA_AUTH_TOKEN_SECRET` environment
variable as described in the section on configuring the hook), the hook will try to fetch the token from the
secret provided in the `opaAuthTokenSecret` (ARN) configuration attribute. Note that the token should be provided
as a plain string in the secret (i.e. the `SecretString`) and not wrapped in a JSON object.

In order to fetch the token, the hook will need to be permitted to perform the `secretsmanager:GetSecretValue`
operation. Note that the hook will **only** read the secret provided by `opaAuthTokenSecret`, but it's recommended
to limit the `HookTypePolicy` on the IAM role to the specific secret accessed, i.e. the same ARN provided in
`opaAuthTokenSecret`. Example `HookTypePolicy` to allow the hook access to a specific secret:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": "secretsmanager:GetSecretValue",
            "Resource": "arn:aws:secretsmanager:eu-north-1:673240551671:secret:opa-cfn-token-l26bHK"
        }
    ]
}
```

If you aren't planning to use bearer tokens for authentication, you may remove the permission entirely.
