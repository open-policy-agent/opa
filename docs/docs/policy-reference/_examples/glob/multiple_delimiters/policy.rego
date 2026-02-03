package example

# Match Docker image references with both / and : delimiters
# Format: registry/image:tag
image_match := glob.match("*/*:*", ["/", ":"], "registry.example.com/library/nginx:latest")
