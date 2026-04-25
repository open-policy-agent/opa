package play

import rego.v1

# An array
containers := ["nginx", "sidecar", "logger"]

container_count := count(containers)

# A set (duplicates don't count twice)
unique_ports := {80, 443, 80}

unique_port_count := count(unique_ports)

# An object (counts the keys)
labels := {
	"app":  "web",
	"tier": "frontend",
	"team": "platform",
}

label_count := count(labels)

# A string (counts the runes)
name := "kubernetes"

name_length := count(name)

# A typical guard: reject if the input has more containers than allowed.
deny contains msg if {
	count(input.spec.containers) > 10
	msg := sprintf("too many containers: %d", [count(input.spec.containers)])
}
