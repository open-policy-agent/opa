---
title: AWS CloudFormation Hook
software:
- aws
- cloudformation
labels:
  type: poweredbyopa
tutorials:
- https://www.openpolicyagent.org/docs/latest/aws-cloudformation-hooks/
code:
- https://github.com/StyraInc/opa-aws-cloudformation-hook
blogs:
- https://blog.styra.com/blog/the-opa-aws-cloudformation-hook
inventors:
- styra
docs_features:
  rest-api-integration:
    note: |
      The OPA CloudFormation Hook uses AWS Lambda to consult an OPA instance
      using the REST API before allowing a CloudFormation stack to be created.

      Read [the tutorial](https://www.openpolicyagent.org/docs/latest/aws-cloudformation-hooks/)
      here in the OPA documentation.
---
AWS CloudFormation Hook that uses OPA to make policy decisions on infrastructure provisioned via AWS CloudFormation
