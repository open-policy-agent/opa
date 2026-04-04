package play

import rego.v1

# Maximum allowed memory in bytes (1 GiB)
max_memory := 1073741824

# Evaluate memory limit violations
violations contains violation if {
    some container in input.containers
    parsed := units.parse(container.memory_limit)
    parsed > max_memory
    violation := {
        "container": container.name,
        "limit": container.memory_limit,
        "parsed_bytes": parsed
    }
}

# Check if all containers are within limits
allow if {
    count(violations) == 0
}
