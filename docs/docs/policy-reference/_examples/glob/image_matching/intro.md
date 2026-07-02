<!-- markdownlint-disable MD041 -->

This example demonstrates when you need multiple delimiters.

Docker image references use multiple separator characters. In this pattern:

- Slash (`/`) separates registry/organization from image name
- Colon (`:`) separates image name from tag

The pattern `*/*:*` matches any registry/organization, image name, and tag. By specifying both `/` and `:` as delimiters, `glob.match` segments the path: `["registry.example.com", "library", "nginx", "latest"]`.
