package example

# Match file paths with / or . separators (e.g., config/app.yaml)
file_path_match := glob.match("config/*.yaml", ["/", "."], "config/app.yaml")

# Match domain names with dot delimiters
domain_match := glob.match("api.*.com", ["."], "api.github.com")

# Match hostnames that contain dashes (e.g., api-v2.example.com)
# This shows when you'd use "-" as a delimiter
hostname_with_dash := glob.match("api-*.example.com", [".", "-"], "api-v2.example.com")

# Match resource paths with : and / (e.g., namespace:deployment/pod)
resource_match := glob.match("*:*/pod", [":", "/"], "kube-system:nginx/pod")

# Match service endpoints with : and / delimiters
endpoint_match := glob.match("*:*/*", [":", "/"], "app:service/endpoint")
