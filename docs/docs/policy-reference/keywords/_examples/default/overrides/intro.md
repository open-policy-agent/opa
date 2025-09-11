<!-- markdownlint-disable MD041 -->
As we saw in the previous example, `default` is helpful for handling undefined
values. Handling undefined values is not just important for callers, but also
within policies themselves.

Using the `default` keyword with functions, we can quickly build in
functionality to set a base case that's overridden when conditions are met.
