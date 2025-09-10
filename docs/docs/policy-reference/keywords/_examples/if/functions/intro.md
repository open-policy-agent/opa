<!-- markdownlint-disable MD041 -->
`if` is also used in functions. Much like rules, in Rego functions can have one
or more heads. The head and body of a function are also separated by the `if`
keyword for consistency and readability.

In this example, we can see that the `is_sudo` function is incrementally defined
where each head adds new cases to the functionality. In this case, each head
defines scenarios where the user is a 'sudoer' - both when the user is an admin
or when the user has the sudo field set.
