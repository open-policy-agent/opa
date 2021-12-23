# Expects policy input as provided by:
# https://api.github.com/repos/open-policy-agent/opa/pulls/${PR_ID}/files
#
# Note that the "filename" here refers to the full path of the file, like
# docs/website/data/integrations.yaml - since that's how it's named in the
# input we'll use the same convention here.

package files

import future.keywords.in

filenames := [f | f := input[_].filename]

changes := {filename: attributes |
	c := input[_]
	filename := c.filename
	attributes := object.remove(c, ["filename"])
}

deny["Logo must be placed in docs/website/static/img/logos/integrations"] {
	"docs/website/data/integrations.yaml" in filenames

	some filename in filenames
	endswith(filename, ".png")
	changes[filename].status == "added"
	directory := substring(filename, 0, last_indexof(filename, "/"))
	directory != "docs/website/static/img/logos/integrations"
}

deny["Logo must be a .png file"] {
	"docs/website/data/integrations.yaml" in filenames

	some filename in filenames
	changes[filename].status == "added"
	directory := substring(filename, 0, last_indexof(filename, "/"))
	directory == "docs/website/static/img/logos/integrations"
	not endswith(filename, ".png")
}

last_indexof(string, search) = i {
	all := [i | chars := split(string, ""); chars[i] == search]
	count(all) > 0
	i := all[count(all) - 1]
} else = -1 {
	true
}
