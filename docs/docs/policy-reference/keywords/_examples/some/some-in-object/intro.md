<!-- markdownlint-disable MD041 -->

Similar to arrays, `some` can also be used on key->value pairs in
objects. Here, we create two variables, one for the key and another
for the value. The Rego rule is then evaluated for each pair.

We can use the key and value however we like. Here, we use the
name of the permission to create a list of permissions that are
toggled on in the `example_object`.
