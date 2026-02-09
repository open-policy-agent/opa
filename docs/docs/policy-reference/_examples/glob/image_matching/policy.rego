package example

# Match Docker image references with both / and : delimiters
# Format: registry/namespace/image:tag
image_match := glob.match("*/*/*:*", ["/", ":"], "registry.example.com/library/nginx:latest")
