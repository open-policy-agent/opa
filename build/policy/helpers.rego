package helpers

import future.keywords.in

last_indexof(string, search) = i {
	all := [i | chars := split(string, ""); chars[i] == search]
	count(all) > 0
	i := all[count(all) - 1]
} else = -1 {
	true
}

endswith_any(string, suffixes) {
	some suffix in suffixes
	endswith(string, suffix)
}
