package play

config := {
	"image": "app:v1",
	"replicas": 3,
	"port": 8080,
}

# object.keys returns the set of top-level keys.
present_keys := object.keys(config)

required_keys := {"image", "replicas"}

# Every required key is present when the required set is a subset of the keys.
has_required if required_keys & present_keys == required_keys
