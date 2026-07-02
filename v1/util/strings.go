package util

import "strings"

// WithPrefix ensures that the string s starts with the given prefix.
// If s already starts with prefix, it is returned unchanged.
func WithPrefix(s, prefix string) string {
	if strings.HasPrefix(s, prefix) {
		return s
	}

	return prefix + s
}
