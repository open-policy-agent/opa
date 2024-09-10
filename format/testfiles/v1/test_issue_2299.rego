package foo

# Compact else cases
authorize = "allow" if {
    input.user == "superuser"
} else = "deny" if {
    input.path[0] == "admin"
    input.source_network == "external"
}

# Newline separated else blocks
q = x if {
    foo == "bar"
}

else = y if {
    foo == "baz"
}

else = z if {
    foo == "qux"
}


# Mixed compact and newline separated
p = x if {
    foo == "bar"
}
# some special case
# with lots of comments
# describing it
else = y if {
    bar == "foo"
} else = z if {
    bar == "bar"
}

else if {
    bar == "baz"
}