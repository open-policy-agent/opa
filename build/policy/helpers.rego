package helpers

import future.keywords.if
import future.keywords.in

last_indexof(string, search) := i if {
	all := [i | chars := split(string, ""); chars[i] == search]
	count(all) > 0
	i := all[count(all) - 1]
} else := -1

basename(filename) := substring(filename, last_indexof(filename, "/") + 1, count(filename) - 1)

extension(filename) := substring(filename, last_indexof(filename, ".") + 1, count(filename) - 1)

directory(filename) := substring(filename, 0, last_indexof(filename, "/"))
