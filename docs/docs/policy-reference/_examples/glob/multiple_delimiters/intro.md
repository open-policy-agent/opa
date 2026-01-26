You can use multiple delimiters in a single `glob.match` call. This is useful when matching paths or data that contain different separator characters.

In this example:
- `file_path_match` uses both `/` and `.` to match file paths like `config/app.yaml`
- `url_match` uses both `:` and `/` to match URLs like `https://api.github.com`
- `data_match` uses both `:` and `/` to match data with mixed separators like `a:b/c`

Each delimiter must still be a single character.
