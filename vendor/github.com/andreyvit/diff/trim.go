package diff

import (
	"strings"
)

// TrimLines applies TrimSpace to each string in the given array.
func TrimLines(input []string) []string {
	result := make([]string, 0, len(input))
	for _, el := range input {
		result = append(result, strings.TrimSpace(el))
	}
	return result
}

// TrimLinesInString applies TrimSpace to each line in the given string, and returns the new trimmed string. Empty lines are not removed.
func TrimLinesInString(input string) string {
	return strings.Join(TrimLines(strings.Split(strings.TrimSpace(input), "\n")), "\n")
}
