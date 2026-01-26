package example

# Match file paths with either / or . separators
file_path_match := glob.match("config/*.yaml", ["/", "."], "config/app.yaml")

# Match domain names with dots and dashes as delimiters
domain_match := glob.match("api.*.com", [".", "-"], "api.github.com")

# Match colon-separated identifiers (like org:module:function)
path_match := glob.match("org:*:function", [":", "/"], "org:module:function")

# Match with both colon and slash delimiters
mixed_match := glob.match("*:*/*", [":", "/"], "app:service/endpoint")
