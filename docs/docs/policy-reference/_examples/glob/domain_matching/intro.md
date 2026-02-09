This example shows basic glob matching with a single delimiter.

The pattern `api.*.com` matches domain names where:

- The first segment is "api"
- Followed by any subdomain (matched by `*`)
- Ending in "com"

The dot (`.`) is specified as the delimiter, which splits the domain into segments: `["api", "example", "com"]`.
