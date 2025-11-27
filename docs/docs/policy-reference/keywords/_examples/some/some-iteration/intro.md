<!-- markdownlint-disable MD041 -->

The `some` keyword can also be used to instantiate variables used in
arbitrary iteration.
This is best practice as it makes it easier for readers
to see which variables are used in each rule.

In the example below, `some` is used to declare `i` and `j`
to search over a 2D array of requests from the past week. This
data is then used to enforce a rate limiting policy on the new
request from a user in the `input`.

Click 'Open in Playground' below to see the full example data.
