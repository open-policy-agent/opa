package foo

# Compact else cases
authorize = "allow" {
    input.user == "superuser"
} else = "deny" {
    input.path[0] == "admin"
    input.source_network == "external"
}

# Newline separated else blocks
q = x {
    foo == "bar"
}

else = y {
    foo == "baz"
}

else = z {
    foo == "qux"
}


# Mixed compact and newline separated
p = x {
    foo == "bar"
}
# some special case
# with lots of comments
# describing it
else = y {
    bar == "foo"
} else = z {
    bar == "bar"
}

else {
    bar == "baz"
}