package foo

authorize = "allow" {
    input.user == "superuser"
} else = "deny" {
    input.path[0] == "admin"
    input.source_network == "external"
}

# base case for p
p = x {
    foo == "bar"
} # some special case
# with lots of comments
# describing it
else = y {
    bar == "foo"
} else = z {
    bar == "bar"
}