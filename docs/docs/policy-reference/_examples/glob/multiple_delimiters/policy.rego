package example

# Match file paths with either / or . separators
file_path_match := glob.match("config/*.yaml", ["/", "."], "config/app.yaml")

# Match domain names with dots and dashes as delimiters
domain_match := glob.match("api.*.com", [".", "-"], "api.github.com")

# Match data with multiple delimiter types
path_match := glob.match("a:*:c", [":", "/"], "a:b:c")

# Match with both colon and slash delimiters
mixed_match := glob.match("*:*/*", [":", "/"], "app:service/endpoint")
