package example

import input.pod

default allow = true

allow = false {
    not pod.metadata.labels.customer
}

allow = false {
    container = pod.spec.containers[_]
    not re_match("^registry.acmecorp.com/.+$", container.image)
}