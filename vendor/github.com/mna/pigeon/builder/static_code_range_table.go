//go:generate go run ../bootstrap/cmd/static_code_generator/main.go -- $GOFILE generated_$GOFILE rangeTable0

package builder

import (
	"fmt"
	"unicode"
)

// IMPORTANT: All code below this line is added to the parser as static code
func rangeTable(class string) *unicode.RangeTable {
	if rt, ok := unicode.Categories[class]; ok {
		return rt
	}
	if rt, ok := unicode.Properties[class]; ok {
		return rt
	}
	if rt, ok := unicode.Scripts[class]; ok {
		return rt
	}

	// cannot happen
	panic(fmt.Sprintf("invalid Unicode class: %s", class))
}
