<!-- markdownlint-disable MD041 -->

Similar to arrays, `some` can also be used on key->value pairs in
objects. Here, two variables are created, one for the key and another
for the value. The Rego rule is then evaluated for each pair.

The key and value can be used in any way. Here, the rule uses the
name of the permission to create a list of permissions that are
toggled on in the `example_object`.
