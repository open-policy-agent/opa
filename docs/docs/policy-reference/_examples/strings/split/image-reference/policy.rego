package play

# Separate the repository from the tag on the ":" separator.
parts := split(input.image, ":")

repository := parts[0]

tag := parts[1]

# Split the repository path into its "/" separated segments.
segments := split(repository, "/")

# The tag must be a valid semver >= 1.2.2.
version_ok if semver.compare(tag, "1.2.2") >= 0
