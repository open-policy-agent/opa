You can use multiple delimiters in a single `glob.match` call. This is useful when matching strings that contain different separator characters.

In this example:
- `file_path_match` uses both `/` and `.` to match file paths like `config/app.yaml`
- `hostname_match` uses both `.` and `-` to match hostnames like `api-v2.example.com` (both dots and dashes are delimiters)
- `resource_match` uses both `:` and `/` to match resource paths like `kube-system:nginx/pod` (namespace:deployment/pod)
- `endpoint_match` uses both `:` and `/` to match service endpoints like `app:service/endpoint`

Each delimiter must still be a single character.
