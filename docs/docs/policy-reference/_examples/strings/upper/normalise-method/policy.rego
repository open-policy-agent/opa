package play

# Upper-case the incoming method so the allow-list check is case-insensitive.
method := upper(input.request.method)

allowed_methods := {"GET", "HEAD", "OPTIONS"}

allow if method in allowed_methods
