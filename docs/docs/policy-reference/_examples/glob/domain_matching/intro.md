<!-- markdownlint-disable MD041 -->

This example shows basic glob matching with a single delimiter.

The pattern `app.*.com` matches domain names where:

- The first segment is "app"
- Followed by any hostname component (matched by `*`)
- Ending in "com"

The dot (`.`) is specified as the delimiter, which splits the domain into segments: `["app", "example", "com"]`.
