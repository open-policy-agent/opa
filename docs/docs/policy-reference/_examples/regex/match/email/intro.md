<!-- markdownlint-disable MD041 -->
Validating emails with Regular Expressions is a common policy task. Email validation
is more complicated than just checking an email matches a pattern, but since a Rego
policy is often a first point of contact, doing a pattern based test on emails is
still a good idea as it can help surface issues to users early if they make a mistake.

`regex.match` is the best way to validate emails in Rego.
