You can use multiple delimiters in a single `glob.match` call. This is useful when matching paths or data that contain different separator characters.

In this example:
- `file_path_match` uses both `/` and `.` to match file paths like `config/app.yaml`
- `domain_match` uses both `.` and `-` to match domain names like `api.github.com`
- `path_match` uses both `:` and `/` to match paths like `a:b:c`
- `mixed_match` uses both `:` and `/` to match mixed data like `app:service/endpoint`

Each delimiter must still be a single character.
