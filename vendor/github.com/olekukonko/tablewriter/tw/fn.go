// Package tw provides utility functions for text formatting, width calculation, and string manipulation
// specifically tailored for table rendering, including handling ANSI escape codes and Unicode text.
package tw

import (
	"bytes"                         // For buffering string output
	"github.com/mattn/go-runewidth" // For calculating display width of Unicode characters
	"math"                          // For mathematical operations like ceiling
	"regexp"                        // For regular expression handling of ANSI codes
	"strconv"                       // For string-to-number conversions
	"strings"                       // For string manipulation utilities
	"unicode"                       // For Unicode character classification
	"unicode/utf8"                  // For UTF-8 rune handling
)

// ansi is a compiled regex pattern used to strip ANSI escape codes.
// These codes are used in terminal output for styling and are invisible in rendered text.
var ansi = CompileANSIFilter()

// CompileANSIFilter constructs and compiles a regex for matching ANSI sequences.
// It supports both control sequences (CSI) and operating system commands (OSC) like hyperlinks.
func CompileANSIFilter() *regexp.Regexp {
	var regESC = "\x1b" // ASCII escape character
	var regBEL = "\x07" // ASCII bell character

	// ANSI string terminator: either ESC+\ or BEL
	var regST = "(" + regexp.QuoteMeta(regESC+"\\") + "|" + regexp.QuoteMeta(regBEL) + ")"
	// Control Sequence Introducer (CSI): ESC[ followed by parameters and a final byte
	var regCSI = regexp.QuoteMeta(regESC+"[") + "[\x30-\x3f]*[\x20-\x2f]*[\x40-\x7e]"
	// Operating System Command (OSC): ESC] followed by arbitrary content until a terminator
	var regOSC = regexp.QuoteMeta(regESC+"]") + ".*?" + regST

	// Combine CSI and OSC patterns into a single regex
	return regexp.MustCompile("(" + regCSI + "|" + regOSC + ")")
}

// DisplayWidth calculates the visual width of a string, excluding ANSI escape sequences.
// It uses go-runewidth to handle Unicode characters correctly.
func DisplayWidth(str string) int {
	// Strip ANSI codes before calculating width to avoid counting invisible characters
	return runewidth.StringWidth(ansi.ReplaceAllLiteralString(str, ""))
}

// TruncateString shortens a string to a specified maximum display width while preserving ANSI color codes.
// An optional suffix (e.g., "...") is appended if truncation occurs.
func TruncateString(s string, maxWidth int, suffix ...string) string {
	// If maxWidth is 0 or negative, return an empty string
	if maxWidth <= 0 {
		return ""
	}

	// Join suffix slices into a single string and calculate its display width
	suffixStr := strings.Join(suffix, " ")
	suffixDisplayWidth := 0
	if len(suffixStr) > 0 {
		// Strip ANSI from suffix for accurate width calculation
		suffixDisplayWidth = runewidth.StringWidth(ansi.ReplaceAllLiteralString(suffixStr, ""))
	}

	// Check if the string (without ANSI) plus suffix fits within maxWidth
	strippedS := ansi.ReplaceAllLiteralString(s, "")
	if runewidth.StringWidth(strippedS)+suffixDisplayWidth <= maxWidth {
		// If it fits, return the original string (with ANSI) plus suffix
		return s + suffixStr
	}

	// Handle edge case: maxWidth is too small for even the suffix
	if maxWidth < suffixDisplayWidth {
		// Try truncating the string without suffix
		return TruncateString(s, maxWidth) // Recursive call without suffix
	}
	// Handle edge case: maxWidth exactly equals suffix width
	if maxWidth == suffixDisplayWidth {
		if runewidth.StringWidth(strippedS) > 0 {
			// If there's content, it's fully truncated; return suffix
			return suffixStr
		}
		return "" // No content and no space for content; return empty string
	}

	// Calculate the maximum width available for the content (excluding suffix)
	targetContentDisplayWidth := maxWidth - suffixDisplayWidth

	var contentBuf bytes.Buffer        // Buffer for building truncated content
	var currentContentDisplayWidth int // Tracks display width of content
	var ansiSeqBuf bytes.Buffer        // Buffer for collecting ANSI sequences
	inAnsiSequence := false            // Tracks if we're inside an ANSI sequence

	// Iterate over runes to build content while respecting maxWidth
	for _, r := range s {
		if r == '\x1b' { // Start of ANSI escape sequence
			if inAnsiSequence {
				// Unexpected new ESC; flush existing sequence
				contentBuf.Write(ansiSeqBuf.Bytes())
				ansiSeqBuf.Reset()
			}
			inAnsiSequence = true
			ansiSeqBuf.WriteRune(r)
		} else if inAnsiSequence {
			ansiSeqBuf.WriteRune(r)
			// Detect end of common ANSI sequences (e.g., SGR 'm' or CSI terminators)
			if r == 'm' || (ansiSeqBuf.Len() > 2 && ansiSeqBuf.Bytes()[1] == '[' && r >= '@' && r <= '~') {
				inAnsiSequence = false
				contentBuf.Write(ansiSeqBuf.Bytes()) // Append completed sequence
				ansiSeqBuf.Reset()
			} else if ansiSeqBuf.Len() > 128 { // Prevent buffer overflow for malformed sequences
				inAnsiSequence = false
				contentBuf.Write(ansiSeqBuf.Bytes())
				ansiSeqBuf.Reset()
			}
		} else {
			// Handle displayable characters
			runeDisplayWidth := runewidth.RuneWidth(r)
			if currentContentDisplayWidth+runeDisplayWidth > targetContentDisplayWidth {
				// Adding this rune would exceed the content width; stop here
				break
			}
			contentBuf.WriteRune(r)
			currentContentDisplayWidth += runeDisplayWidth
		}
	}

	// Append any unterminated ANSI sequence
	if ansiSeqBuf.Len() > 0 {
		contentBuf.Write(ansiSeqBuf.Bytes())
	}

	finalContent := contentBuf.String()

	// Append suffix if content was truncated or if suffix is provided and content exists
	if runewidth.StringWidth(ansi.ReplaceAllLiteralString(finalContent, "")) < runewidth.StringWidth(strippedS) {
		// Content was truncated; append suffix
		return finalContent + suffixStr
	} else if len(suffixStr) > 0 && len(finalContent) > 0 {
		// No truncation but suffix exists; append it
		return finalContent + suffixStr
	} else if len(suffixStr) > 0 && len(strippedS) == 0 {
		// Original string was empty; return suffix
		return suffixStr
	}

	// Return content as is (with preserved ANSI codes)
	return finalContent
}

// Title normalizes and uppercases a label string for use in headers.
// It replaces underscores and certain dots with spaces and trims whitespace.
func Title(name string) string {
	origLen := len(name)
	rs := []rune(name)
	for i, r := range rs {
		switch r {
		case '_':
			rs[i] = ' ' // Replace underscores with spaces
		case '.':
			// Replace dots with spaces unless they are between numeric or space characters
			if (i != 0 && !IsIsNumericOrSpace(rs[i-1])) || (i != len(rs)-1 && !IsIsNumericOrSpace(rs[i+1])) {
				rs[i] = ' '
			}
		}
	}
	name = string(rs)
	name = strings.TrimSpace(name)
	// If the input was non-empty but trimmed to empty, return a single space
	if len(name) == 0 && origLen > 0 {
		name = " "
	}
	// Convert to uppercase for header formatting
	return strings.ToUpper(name)
}

// PadCenter centers a string within a specified width using a padding character.
// Extra padding is split between left and right, with slight preference to left if uneven.
func PadCenter(s, pad string, width int) string {
	gap := width - DisplayWidth(s)
	if gap > 0 {
		// Calculate left and right padding; ceil ensures left gets extra if gap is odd
		gapLeft := int(math.Ceil(float64(gap) / 2))
		gapRight := gap - gapLeft
		return strings.Repeat(pad, gapLeft) + s + strings.Repeat(pad, gapRight)
	}
	// If no padding needed or string is too wide, return as is
	return s
}

// PadRight left-aligns a string within a specified width, filling remaining space on the right with padding.
func PadRight(s, pad string, width int) string {
	gap := width - DisplayWidth(s)
	if gap > 0 {
		// Append padding to the right
		return s + strings.Repeat(pad, gap)
	}
	// If no padding needed or string is too wide, return as is
	return s
}

// PadLeft right-aligns a string within a specified width, filling remaining space on the left with padding.
func PadLeft(s, pad string, width int) string {
	gap := width - DisplayWidth(s)
	if gap > 0 {
		// Prepend padding to the left
		return strings.Repeat(pad, gap) + s
	}
	// If no padding needed or string is too wide, return as is
	return s
}

// Pad aligns a string within a specified width using a padding character.
// It truncates if the string is wider than the target width.
func Pad(s string, padChar string, totalWidth int, alignment Align) string {
	sDisplayWidth := DisplayWidth(s)
	if sDisplayWidth > totalWidth {
		return TruncateString(s, totalWidth) // Only truncate if necessary
	}
	switch alignment {
	case AlignLeft:
		return PadRight(s, padChar, totalWidth)
	case AlignRight:
		return PadLeft(s, padChar, totalWidth)
	case AlignCenter:
		return PadCenter(s, padChar, totalWidth)
	default:
		return PadRight(s, padChar, totalWidth)
	}
}

// IsIsNumericOrSpace checks if a rune is a digit or space character.
// Used in formatting logic to determine safe character replacements.
func IsIsNumericOrSpace(r rune) bool {
	return ('0' <= r && r <= '9') || r == ' '
}

// IsNumeric checks if a string represents a valid integer or floating-point number.
func IsNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	// Try parsing as integer first
	if _, err := strconv.Atoi(s); err == nil {
		return true
	}
	// Then try parsing as float
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// SplitCamelCase splits a camelCase or PascalCase or snake_case string into separate words.
// It detects transitions between uppercase, lowercase, digits, and other characters.
func SplitCamelCase(src string) (entries []string) {
	// Validate UTF-8 input; return as single entry if invalid
	if !utf8.ValidString(src) {
		return []string{src}
	}
	entries = []string{}
	var runes [][]rune
	lastClass := 0
	class := 0
	// Classify each rune into categories: lowercase (1), uppercase (2), digit (3), other (4)
	for _, r := range src {
		switch {
		case unicode.IsLower(r):
			class = 1
		case unicode.IsUpper(r):
			class = 2
		case unicode.IsDigit(r):
			class = 3
		default:
			class = 4
		}
		// Group consecutive runes of the same class together
		if class == lastClass {
			runes[len(runes)-1] = append(runes[len(runes)-1], r)
		} else {
			runes = append(runes, []rune{r})
		}
		lastClass = class
	}
	// Adjust for cases where an uppercase letter is followed by lowercase (e.g., CamelCase)
	for i := 0; i < len(runes)-1; i++ {
		if unicode.IsUpper(runes[i][0]) && unicode.IsLower(runes[i+1][0]) {
			// Move the last uppercase rune to the next group for proper word splitting
			runes[i+1] = append([]rune{runes[i][len(runes[i])-1]}, runes[i+1]...)
			runes[i] = runes[i][:len(runes[i])-1]
		}
	}
	// Convert rune groups to strings, excluding empty, underscore or whitespace-only groups
	for _, s := range runes {
		str := string(s)
		if len(s) > 0 && strings.TrimSpace(str) != "" && str != "_" {
			entries = append(entries, str)
		}
	}
	return
}

// Or provides a ternary-like operation for strings, returning 'valid' if cond is true, else 'inValid'.
func Or(cond bool, valid, inValid string) string {
	if cond {
		return valid
	}
	return inValid
}

// Max returns the greater of two integers.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Min returns the smaller of two integers.
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// BreakPoint finds the rune index where the display width of a string first exceeds the specified limit.
// It returns the number of runes if the entire string fits, or 0 if nothing fits.
func BreakPoint(s string, limit int) int {
	// If limit is 0 or negative, nothing can fit
	if limit <= 0 {
		return 0
	}
	// Empty string has a breakpoint of 0
	if s == "" {
		return 0
	}

	currentWidth := 0
	runeCount := 0
	// Iterate over runes, accumulating display width
	for _, r := range s {
		runeWidth := DisplayWidth(string(r)) // Calculate width of individual rune
		if currentWidth+runeWidth > limit {
			// Adding this rune would exceed the limit; breakpoint is before this rune
			if currentWidth == 0 {
				// First rune is too wide; allow breaking after it if limit > 0
				if runeWidth > limit && limit > 0 {
					return 1
				}
				return 0
			}
			return runeCount
		}
		currentWidth += runeWidth
		runeCount++
	}

	// Entire string fits within the limit
	return runeCount
}

func MakeAlign(l int, align Align) Alignment {
	aa := make(Alignment, l)
	for i := 0; i < l; i++ {
		aa[i] = align
	}
	return aa
}
