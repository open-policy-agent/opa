package play

config := {
	"image": "app:v1",
	"replicas": 3,
	"port": 8080,
}

# object.keys returns the set of top-level keys.
config_keys := object.keys(config)

# Every key must be lowercase. Compare each key to its lowered form.
all_lowercase if {
	every k in config_keys {
		lower(k) == k
	}
}
