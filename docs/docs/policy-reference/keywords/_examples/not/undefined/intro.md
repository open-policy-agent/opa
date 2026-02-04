<!-- markdownlint-disable MD041 -->

One of the most important use cases for `not` is checking for undefined values.
In this example a policy uses `not` to deny any request without an `email` set.
Even if a value is not used in the policy, it might be important information for
the [decision log](/docs/management-decision-logs).

Try updating the example `input.json`, changing `e_mail` to `email`. When
`e_mail` is set, then `email` is undefined and `not` checks for that in the
first rule.
