<!-- markdownlint-disable MD041 -->
While this first example is trivially simple and unlikely to be useful when building real
policies, it illustrates the fundamental reason for using the `contains` keyword: building sets.
Sets are unordered collections, and they form an important building block for many policies.

In this example, we use a multi-value rule defined using the `contains` keyword to create a simple
list of todos. Just remember that sets are unordered and so you should not depend on the order of
the result.
