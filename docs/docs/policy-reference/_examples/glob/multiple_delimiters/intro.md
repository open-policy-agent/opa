This example demonstrates when you need multiple delimiters.

Resource paths in systems like Kubernetes use multiple separator characters. In this pattern:
- Colon (`:`) separates namespace from resource name
- Slash (`/`) separates resource name from type

The pattern `*:*/pod` matches any namespace and resource name, but specifically resources of type "pod". By specifying both `:` and `/` as delimiters, `glob.match` segments the path: `["kube-system", "nginx", "pod"]`.
