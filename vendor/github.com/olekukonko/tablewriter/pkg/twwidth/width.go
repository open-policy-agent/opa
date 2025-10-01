package twwidth

import (
	"bytes"
	"regexp"
	"strings"
	"sync"

	"github.com/mattn/go-runewidth"
)

// condition holds the global runewidth configuration, including East Asian width settings.
var condition *runewidth.Condition

// mu protects access to condition and widthCache for thread safety.
var mu sync.Mutex

// ansi is a compiled regular expression for stripping ANSI escape codes from strings.
var ansi = Filter()

func init() {
	condition = runewidth.NewCondition()
	widthCache = make(map[cacheKey]int)
}

// cacheKey is used as a key for memoizing string width results in widthCache.
type cacheKey struct {
	str            string // Input string
	eastAsianWidth bool   // East Asian width setting
}

// widthCache stores memoized results of Width calculations to improve performance.
var widthCache map[cacheKey]int

// Filter compiles and returns a regular expression for matching ANSI escape sequences,
// including CSI (Control Sequence Introducer) and OSC (Operating System Command) sequences.
// The returned regex can be used to strip ANSI codes from strings.
func Filter() *regexp.Regexp {
	regESC := "\x1b" // ASCII escape character
	regBEL := "\x07" // ASCII bell character

	// ANSI string terminator: either ESC+\ or BEL
	regST := "(" + regexp.QuoteMeta(regESC+"\\") + "|" + regexp.QuoteMeta(regBEL) + ")"
	// Control Sequence Introducer (CSI): ESC[ followed by parameters and a final byte
	regCSI := regexp.QuoteMeta(regESC+"[") + "[\x30-\x3f]*[\x20-\x2f]*[\x40-\x7e]"
	// Operating System Command (OSC): ESC] followed by arbitrary content until a terminator
	regOSC := regexp.QuoteMeta(regESC+"]") + ".*?" + regST

	// Combine CSI and OSC patterns into a single regex
	return regexp.MustCompile("(" + regCSI + "|" + regOSC + ")")
}

// SetEastAsian enables or disables East Asian width handling for width calculations.
// When the setting changes, the width cache is cleared to ensure accuracy.
// This function is thread-safe.
//
// Example:
//
//	twdw.SetEastAsian(true) // Enable East Asian width handling
func SetEastAsian(enable bool) {
	mu.Lock()
	defer mu.Unlock()
	if condition.EastAsianWidth != enable {
		condition.EastAsianWidth = enable
		widthCache = make(map[cacheKey]int) // Clear cache on setting change
	}
}

// SetCondition updates the global runewidth.Condition used for width calculations.
// When the condition is changed, the width cache is cleared.
// This function is thread-safe.
//
// Example:
//
//	newCond := runewidth.NewCondition()
//	newCond.EastAsianWidth = true
//	twdw.SetCondition(newCond)
func SetCondition(newCond *runewidth.Condition) {
	mu.Lock()
	defer mu.Unlock()
	condition = newCond
	widthCache = make(map[cacheKey]int) // Clear cache on setting change
}

// Width calculates the visual width of a string, excluding ANSI escape sequences,
// using the go-runewidth package for accurate Unicode handling. It accounts for the
// current East Asian width setting and caches results for performance.
// This function is thread-safe.
//
// Example:
//
//	width := twdw.Width("Hello\x1b[31mWorld") // Returns 10
func Width(str string) int {
	mu.Lock()
	key := cacheKey{str: str, eastAsianWidth: condition.EastAsianWidth}
	if w, found := widthCache[key]; found {
		mu.Unlock()
		return w
	}
	mu.Unlock()

	// Use a temporary condition to avoid holding the lock during calculation
	tempCond := runewidth.NewCondition()
	tempCond.EastAsianWidth = key.eastAsianWidth

	stripped := ansi.ReplaceAllLiteralString(str, "")
	calculatedWidth := tempCond.StringWidth(stripped)

	mu.Lock()
	widthCache[key] = calculatedWidth
	mu.Unlock()

	return calculatedWidth
}

// WidthNoCache calculates the visual width of a string without using or
// updating the global cache. It uses the current global East Asian width setting.
// This function is intended for internal use (e.g., benchmarking) and is thread-safe.
//
// Example:
//
//	width := twdw.WidthNoCache("Hello\x1b[31mWorld") // Returns 10
func WidthNoCache(str string) int {
	mu.Lock()
	currentEA := condition.EastAsianWidth
	mu.Unlock()

	tempCond := runewidth.NewCondition()
	tempCond.EastAsianWidth = currentEA

	stripped := ansi.ReplaceAllLiteralString(str, "")
	return tempCond.StringWidth(stripped)
}

// Display calculates the visual width of a string, excluding ANSI escape sequences,
// using the provided runewidth condition. Unlike Width, it does not use caching
// and is intended for cases where a specific condition is required.
// This function is thread-safe with respect to the provided condition.
//
// Example:
//
//	cond := runewidth.NewCondition()
//	width := twdw.Display(cond, "Hello\x1b[31mWorld") // Returns 10
func Display(cond *runewidth.Condition, str string) int {
	return cond.StringWidth(ansi.ReplaceAllLiteralString(str, ""))
}

// Truncate shortens a string to fit within a specified visual width, optionally
// appending a suffix (e.g., "..."). It preserves ANSI escape sequences and adds
// a reset sequence (\x1b[0m) if needed to prevent formatting bleed. The function
// respects the global East Asian width setting and is thread-safe.
//
// If maxWidth is negative, an empty string is returned. If maxWidth is zero and
// a suffix is provided, the suffix is returned. If the string's visual width is
// less than or equal to maxWidth, the string (and suffix, if provided and fits)
// is returned unchanged.
//
// Example:
//
//	s := twdw.Truncate("Hello\x1b[31mWorld", 5, "...") // Returns "Hello..."
//	s = twdw.Truncate("Hello", 10) // Returns "Hello"
func Truncate(s string, maxWidth int, suffix ...string) string {
	if maxWidth < 0 {
		return ""
	}

	suffixStr := strings.Join(suffix, "")
	sDisplayWidth := Width(s)              // Uses global cached Width
	suffixDisplayWidth := Width(suffixStr) // Uses global cached Width

	// Case 1: Original string is visually empty.
	if sDisplayWidth == 0 {
		// If suffix is provided and fits within maxWidth (or if maxWidth is generous)
		if len(suffixStr) > 0 && suffixDisplayWidth <= maxWidth {
			return suffixStr
		}
		// If s has ANSI codes (len(s)>0) but maxWidth is 0, can't display them.
		if maxWidth == 0 && len(s) > 0 {
			return ""
		}
		return s // Returns "" or original ANSI codes
	}

	// Case 2: maxWidth is 0, but string has content. Cannot display anything.
	if maxWidth == 0 {
		return ""
	}

	// Case 3: String fits completely or fits with suffix.
	// Here, maxWidth is the total budget for the line.
	if sDisplayWidth <= maxWidth {
		if len(suffixStr) == 0 { // No suffix.
			return s
		}
		// Suffix is provided. Check if s + suffix fits.
		if sDisplayWidth+suffixDisplayWidth <= maxWidth {
			return s + suffixStr
		}
		// s fits, but s + suffix is too long. Return s.
		return s
	}

	// Case 4: String needs truncation (sDisplayWidth > maxWidth).
	// maxWidth is the total budget for the final string (content + suffix).

	// Capture the global EastAsianWidth setting once for consistent use
	mu.Lock()
	currentGlobalEastAsianWidth := condition.EastAsianWidth
	mu.Unlock()

	// Special case for EastAsian true: if only suffix fits, return suffix.
	// This was derived from previous test behavior.
	if len(suffixStr) > 0 && currentGlobalEastAsianWidth {
		provisionalContentWidth := maxWidth - suffixDisplayWidth
		if provisionalContentWidth == 0 { // Exactly enough space for suffix only
			return suffixStr // <<<< MODIFIED: No ANSI reset here
		}
	}

	// Calculate the budget for the content part, reserving space for the suffix.
	targetContentForIteration := maxWidth
	if len(suffixStr) > 0 {
		targetContentForIteration -= suffixDisplayWidth
	}

	// If content budget is negative, means not even suffix fits (or no suffix and no space).
	// However, if only suffix fits, it should be handled.
	if targetContentForIteration < 0 {
		// Can we still fit just the suffix?
		if len(suffixStr) > 0 && suffixDisplayWidth <= maxWidth {
			if strings.Contains(s, "\x1b[") {
				return "\x1b[0m" + suffixStr
			}
			return suffixStr
		}
		return "" // Cannot fit anything.
	}
	// If targetContentForIteration is 0, loop won't run, result will be empty string, then suffix is added.

	var contentBuf bytes.Buffer
	var currentContentDisplayWidth int
	var ansiSeqBuf bytes.Buffer
	inAnsiSequence := false
	ansiWrittenToContent := false

	localRunewidthCond := runewidth.NewCondition()
	localRunewidthCond.EastAsianWidth = currentGlobalEastAsianWidth

	for _, r := range s {
		if r == '\x1b' {
			inAnsiSequence = true
			ansiSeqBuf.Reset()
			ansiSeqBuf.WriteRune(r)
		} else if inAnsiSequence {
			ansiSeqBuf.WriteRune(r)
			seqBytes := ansiSeqBuf.Bytes()
			seqLen := len(seqBytes)
			terminated := false
			if seqLen >= 2 {
				introducer := seqBytes[1]
				switch introducer {
				case '[':
					if seqLen >= 3 && r >= 0x40 && r <= 0x7E {
						terminated = true
					}
				case ']':
					if r == '\x07' {
						terminated = true
					} else if seqLen > 1 && seqBytes[seqLen-2] == '\x1b' && r == '\\' { // Check for ST: \x1b\
						terminated = true
					}
				}
			}
			if terminated {
				inAnsiSequence = false
				contentBuf.Write(ansiSeqBuf.Bytes())
				ansiWrittenToContent = true
				ansiSeqBuf.Reset()
			}
		} else { // Normal character
			runeDisplayWidth := localRunewidthCond.RuneWidth(r)
			if targetContentForIteration == 0 { // No budget for content at all
				break
			}
			if currentContentDisplayWidth+runeDisplayWidth > targetContentForIteration {
				break
			}
			contentBuf.WriteRune(r)
			currentContentDisplayWidth += runeDisplayWidth
		}
	}

	result := contentBuf.String()

	// Suffix is added if:
	// 1. A suffix string is provided.
	// 2. Truncation actually happened (sDisplayWidth > maxWidth originally)
	//    OR if the content part is empty but a suffix is meant to be shown
	//    (e.g. targetContentForIteration was 0).
	if len(suffixStr) > 0 {
		// Add suffix if we are in the truncation path (sDisplayWidth > maxWidth)
		// OR if targetContentForIteration was 0 (meaning only suffix should be shown)
		// but we must ensure we don't exceed original maxWidth.
		// The logic above for targetContentForIteration already ensures space.

		needsReset := false
		// Condition for reset: if styling was active in 's' and might affect suffix
		if (ansiWrittenToContent || (inAnsiSequence && strings.Contains(s, "\x1b["))) && (currentContentDisplayWidth > 0 || ansiWrittenToContent) {
			if !strings.HasSuffix(result, "\x1b[0m") {
				needsReset = true
			}
		} else if currentContentDisplayWidth > 0 && strings.Contains(result, "\x1b[") && !strings.HasSuffix(result, "\x1b[0m") && strings.Contains(s, "\x1b[") {
			// If result has content and ANSI, and original had ANSI, and result not already reset
			needsReset = true
		}

		if needsReset {
			result += "\x1b[0m"
		}
		result += suffixStr
	}
	return result
}
