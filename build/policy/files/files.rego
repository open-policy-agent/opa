# METADATA
# description: |
#   Expects policy input as provided by:
#   https://api.github.com/repos/open-policy-agent/opa/pulls/${PR_ID}/files
#
#   Note that the "filename" here refers to the full, relative path of the file,
#   like docs/foo/bar.yaml - since that's how it's named in the input we'll use
#   the same convention here.
# schemas:
#   - input: schema.files
package files

# METADATA
# entrypoint: true
deny contains sprintf("%s is an invalid YAML file: %s", [filename, content]) if {
	some filename, content in yaml_file_contents
	changes[filename].status in {"added", "modified"}
	not yaml.is_valid(content)
}

deny contains sprintf("%s is an invalid JSON file: %s", [filename, content]) if {
	some filename, content in json_file_contents
	changes[filename].status in {"added", "modified"}
	not json.is_valid(content)
}

yaml_file_contents[filename] := file_in_pr(filename) if {
	some filename in filenames
	extension(filename) in {"yml", "yaml"}
}

json_file_contents[filename] := file_in_pr(filename) if {
	some filename in filenames
	extension(filename) == "json"
}

filenames contains f.filename if some f in input

changes[filename] := attributes if {
	some change in input
	filename := change.filename
	attributes := object.remove(change, ["filename"])
}

http_error(response) if response.status_code == 0
http_error(response) if response.status_code >= 400

dump_response_on_error(response) := response if {
	http_error(response)
	print("unexpected error in response", response) # regal ignore:print-or-trace-call
}

dump_response_on_error(response) := response if not http_error(response)

file_in_pr(filename) := dump_response_on_error(http.send({
	"url": changes[filename].raw_url,
	"method": "GET",
	"headers": {"Authorization": sprintf("Bearer %v", [opa.runtime().env.GITHUB_TOKEN])},
	"cache": true,
	"enable_redirect": true,
	"raise_error": false,
})).raw_body

default last_indexof(_, _) := -1

last_indexof(string, search) := indices[count(indices) - 1] if indices := indexof_n(string, search)

extension(filename) := substring(filename, last_indexof(filename, ".") + 1, count(filename) - 1)
