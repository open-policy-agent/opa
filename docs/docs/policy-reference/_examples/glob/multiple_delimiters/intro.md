You can use multiple delimiters in a single `glob.match` call. This is useful when matching strings that contain different separator characters.

In this example:
- `file_path_match` uses both `/` and `.` to match file paths like `config/app.yaml`
- `domain_match` uses `.` to match simple domain names like `api.github.com`
- `hostname_with_dash` demonstrates when you'd use `-` as a delimiter: matching hostnames like `api-v2.example.com` that contain dashes
- `resource_match` uses both `:` and `/` to match Kubernetes-style resource paths like `kube-system:nginx/pod`
- `endpoint_match` uses both `:` and `/` to match service endpoints like `app:service/endpoint`

Each delimiter must still be a single character.
