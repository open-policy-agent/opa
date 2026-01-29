package example

# Match domain names using dot as delimiter
domain_match := glob.match("api.*.com", ["."], "api.github.com")
