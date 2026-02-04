<!-- markdownlint-disable MD041 -->

When default deny behavior is required, knowing that a value will never be
undefined is helpful. This is common in access control systems where access is
denied unless explicitly allowed.

In the following example, the policy's allow rules depend on fields in `input`.
If any field is missing, `allow` should return false instead of undefined. This
is achieved using the `default` keyword.

The policy handles unexpected data formats, ensuring the result is always a
boolean.
