package a

setting := {
    # set to enable
    "enabled": true,
}

setting2 := {
    # set to enable
    # or disable
    # who knows?
    "enabled": true,
}

p := {
    "foo": "bar" # baz
}

q := {
    "foo" # bar
}

# beforeEnd comment on the opening brace line, plus an interior comment.
# Without clearing beforeEnd on unexpectedCommentError, the inline
# comment would be written twice.
s := { # inline
    # interior
    "key": "value",
}
