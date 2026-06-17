package play

# The naming convention requires every configuration key to be lowercase.
config_keys := object.keys(input.config)

# Collect the keys that break the convention.
invalid_keys := {key | some key in config_keys; lower(key) != key}

# The config is valid only when no offending keys remain.
keys_valid := count(invalid_keys) == 0
