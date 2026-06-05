package play

user := {
	"name": "alice",
	"roles": ["dev", "ops"],
}

# The fallback team is derived from the user's first role rather than being a
# fixed literal, the kind of computed default the `default` keyword cannot express.
fallback_team := user.roles[0]

# "team" is absent, so object.get returns the computed fallback instead of an
# undefined result.
team := object.get(user, "team", fallback_team)
