package example

# METADATA
# title: Deny invalid numbers
# description: Numbers may not be higher than 5
# custom:
#  severity: MEDIUM
deny contains format(rego.metadata.rule()) if {
        input.number > 5
}

# METADATA
# title: Deny non-admin subjects
# description: Subject must have the 'admin' role
# custom:
#  severity: HIGH
deny contains format(rego.metadata.rule()) if {
        input.subject.role != "admin"
}

format(meta) := {"severity": meta.custom.severity, "reason": meta.description}
