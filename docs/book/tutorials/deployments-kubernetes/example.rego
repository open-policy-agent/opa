package example

default deny = false

# Reject objects without a customer label.
deny {
    not input.metadata.labels.customer
}

# Reject pods referring to images outside the corporate registry.
deny {
    input.kind == "Pod"
    container := input.spec.containers[_]
    not re_match("^registry.acmecorp.com/.+$", container.image)
}