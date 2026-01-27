package example

# Match file paths with either / or . separators
file_path_match := glob.match("config/*.yaml", ["/", "."], "config/app.yaml")

# Match hostnames with dots and dashes (e.g., api-v2.example.com)
hostname_match := glob.match("api-*.example.com", [".", "-"], "api-v2.example.com")

# Match resource paths with colons and slashes (e.g., namespace:deployment/pod)
resource_match := glob.match("*:*/pod", [":", "/"], "kube-system:nginx/pod")

# Match service endpoints with both colon and slash delimiters
endpoint_match := glob.match("*:*/*", [":", "/"], "app:service/endpoint")
