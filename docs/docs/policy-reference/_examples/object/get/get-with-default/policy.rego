package play

user := {
	"name": "alice",
	"roles": ["dev", "ops"],
}

# "name" is present, so its value is returned.
name := object.get(user, "name", "unknown")

# "team" is absent, so the default is returned instead of an undefined result.
team := object.get(user, "team", "unassigned")
