package example

# Match file paths with either / or . separators
file_path_match := glob.match("config/*.yaml", ["/", "."], "config/app.yaml")

# Match URLs with : and / separators
url_match := glob.match("https:*github.com", [":", "/"], "https://api.github.com")

# Match mixed delimiters in data
data_match := glob.match("a*c", [":", "/"], "a:b/c")
