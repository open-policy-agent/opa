package play

image := "registry.example.com/team/app:v1.2.3"

# Separate the repository from the tag on the ":" separator.
parts := split(image, ":")

repository := parts[0]

tag := parts[1]

# Split the repository path into its "/" separated segments.
segments := split(repository, "/")
