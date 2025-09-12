---
title: Custom
sidebar_position: 7
---


# Custom

The `custom` category is a special one, as the rules in this category allow you
to enforce rules that are specific to your project, team or organization. This
typically includes things like naming conventions, where you might want to
ensure that, for example, all package names adhere to an organizational
standard, like having a prefix matching the organization name.

:::warning
Since these rules require configuration provided by the user, or are more
opinionated than other rules, they are disabled by default. In order to enable
them, see the configuration options available for each rule for how to configure
them according to your requirements.
:::

For more advanced requirements, see the guide on writing [custom rules](https://openpolicyagent.org/projects/regal/custom-rules) in Rego.

import RulesTable from '@site/src/components/projects/regal/RulesTable';

<!-- markdownlint-disable MD033 -->
<RulesTable category="custom"/>
