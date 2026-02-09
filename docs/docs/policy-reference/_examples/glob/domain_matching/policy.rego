package example

# Match domain names using dot as delimiter
domain_match := glob.match("app.*.com", ["."], "app.example.com")
