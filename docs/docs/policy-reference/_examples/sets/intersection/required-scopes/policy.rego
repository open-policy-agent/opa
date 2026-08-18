package play

granted := {s | some s in input.granted}
required := {s | some s in input.required}

present := intersection({granted, required})

missing := required - present

default allow := false

allow if count(missing) == 0

deny contains msg if {
	count(missing) > 0
	msg := sprintf("missing required scopes: %v", [missing])
}
