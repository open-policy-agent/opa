package example

# Match resource paths with both : and / delimiters
# Format: namespace:resource/type
resource_match := glob.match("*:*/pod", [":", "/"], "kube-system:nginx/pod")
