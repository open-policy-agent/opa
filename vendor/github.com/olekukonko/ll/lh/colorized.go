package lh

import (
	"fmt"
	"github.com/olekukonko/ll/lx"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// Palette defines ANSI color codes for various log components.
// It specifies colors for headers, goroutines, functions, paths, stack traces, and log levels,
// used by ColorizedHandler to format log output with color.
type Palette struct {
	Header    string // Color for stack trace header and dump separators
	Goroutine string // Color for goroutine lines in stack traces
	Func      string // Color for function names in stack traces
	Path      string // Color for file paths in stack traces
	FileLine  string // Color for file line numbers (not used in provided code)
	Reset     string // Reset code to clear color formatting
	Pos       string // Color for position in hex dumps
	Hex       string // Color for hex values in dumps
	Ascii     string // Color for ASCII values in dumps
	Debug     string // Color for Debug level messages
	Info      string // Color for Info level messages
	Warn      string // Color for Warn level messages
	Error     string // Color for Error level messages
	Title     string // Color for dump titles (BEGIN/END separators)
}

// darkPalette defines colors optimized for dark terminal backgrounds.
// It uses bright, contrasting colors for readability on dark backgrounds.
var darkPalette = Palette{
	Header:    "\033[1;31m",     // Bold red for headers
	Goroutine: "\033[1;36m",     // Bold cyan for goroutines
	Func:      "\033[97m",       // Bright white for functions
	Path:      "\033[38;5;245m", // Light gray for paths
	FileLine:  "\033[38;5;111m", // Muted light blue (unused)
	Reset:     "\033[0m",        // Reset color formatting

	Title: "\033[38;5;245m", // Light gray for dump titles
	Pos:   "\033[38;5;117m", // Light blue for dump positions
	Hex:   "\033[38;5;156m", // Light green for hex values
	Ascii: "\033[38;5;224m", // Light pink for ASCII values

	Debug: "\033[36m", // Cyan for Debug level
	Info:  "\033[32m", // Green for Info level
	Warn:  "\033[33m", // Yellow for Warn level
	Error: "\033[31m", // Red for Error level
}

// lightPalette defines colors optimized for light terminal backgrounds.
// It uses darker colors for better contrast on light backgrounds.
var lightPalette = Palette{
	Header:    "\033[1;31m", // Same red for headers
	Goroutine: "\033[34m",   // Blue (darker for light bg)
	Func:      "\033[30m",   // Black text for functions
	Path:      "\033[90m",   // Dark gray for paths
	FileLine:  "\033[94m",   // Blue for file lines (unused)
	Reset:     "\033[0m",    // Reset color formatting

	Title: "\033[38;5;245m", // Light gray for dump titles
	Pos:   "\033[38;5;117m", // Light blue for dump positions
	Hex:   "\033[38;5;156m", // Light green for hex values
	Ascii: "\033[38;5;224m", // Light pink for ASCII values

	Debug: "\033[36m", // Cyan for Debug level
	Info:  "\033[32m", // Green for Info level
	Warn:  "\033[33m", // Yellow for Warn level
	Error: "\033[31m", // Red for Error level
}

// ColorizedHandler is a handler that outputs log entries with ANSI color codes.
// It formats log entries with colored namespace, level, message, fields, and stack traces,
// writing the result to the provided writer.
// Thread-safe if the underlying writer is thread-safe.
type ColorizedHandler struct {
	w          io.Writer // Destination for colored log output
	palette    Palette   // Color scheme for formatting
	showTime   bool      // Whether to display timestamps
	timeFormat string    // Format for timestamps (defaults to time.RFC3339)
}

// ColorOption defines a configuration function for ColorizedHandler.
// It allows customization of the handler, such as setting the color palette.
type ColorOption func(*ColorizedHandler)

// WithColorPallet sets the color palette for the ColorizedHandler.
// It allows specifying a custom Palette for dark or light terminal backgrounds.
// Example:
//
//	handler := NewColorizedHandler(os.Stdout, WithColorPallet(lightPalette))
func WithColorPallet(pallet Palette) ColorOption {
	return func(c *ColorizedHandler) {
		c.palette = pallet
	}
}

// NewColorizedHandler creates a new ColorizedHandler writing to the specified writer.
// It initializes the handler with a detected or specified color palette and applies
// optional configuration functions.
// Example:
//
//	handler := NewColorizedHandler(os.Stdout)
//	logger := ll.New("app").Enable().Handler(handler)
//	logger.Info("Test") // Output: [app] <colored INFO>: Test
func NewColorizedHandler(w io.Writer, opts ...ColorOption) *ColorizedHandler {
	// Initialize with writer
	c := &ColorizedHandler{w: w,
		showTime:   false,
		timeFormat: time.RFC3339,
	}

	// Apply configuration options
	for _, opt := range opts {
		opt(c)
	}
	// Detect palette if not set
	c.palette = c.detectPalette()
	return c
}

// Handle processes a log entry and writes it with ANSI color codes.
// It delegates to specialized methods based on the entry's class (Dump, Raw, or regular).
// Returns an error if writing to the underlying writer fails.
// Thread-safe if the writer is thread-safe.
// Example:
//
//	handler.Handle(&lx.Entry{Message: "test", Level: lx.LevelInfo}) // Writes colored output
func (h *ColorizedHandler) Handle(e *lx.Entry) error {
	switch e.Class {
	case lx.ClassDump:
		// Handle hex dump entries
		return h.handleDumpOutput(e)
	case lx.ClassRaw:
		// Write raw entries directly
		_, err := h.w.Write([]byte(e.Message))
		return err
	default:
		// Handle standard log entries
		return h.handleRegularOutput(e)
	}
}

// Timestamped enables or disables timestamp display and optionally sets a custom time format.
// If format is empty, defaults to RFC3339.
// Example:
//
//	handler := NewColorizedHandler(os.Stdout).Timestamped(true, time.StampMilli)
//	// Output: Jan 02 15:04:05.000 [app] INFO: Test
func (h *ColorizedHandler) Timestamped(enable bool, format ...string) {
	h.showTime = enable
	if len(format) > 0 && format[0] != "" {
		h.timeFormat = format[0]
	}
}

// handleRegularOutput handles normal log entries.
// It formats the entry with colored namespace, level, message, fields, and stack trace (if present),
// writing the result to the handler's writer.
// Returns an error if writing fails.
// Example (internal usage):
//
//	h.handleRegularOutput(&lx.Entry{Message: "test", Level: lx.LevelInfo}) // Writes colored output
func (h *ColorizedHandler) handleRegularOutput(e *lx.Entry) error {
	var builder strings.Builder // Buffer for building formatted output

	// Add timestamp if enabled
	if h.showTime {
		builder.WriteString(e.Timestamp.Format(h.timeFormat))
		builder.WriteString(lx.Space)
	}

	// Format namespace with colors
	h.formatNamespace(&builder, e)

	// Format level with color based on severity
	h.formatLevel(&builder, e)

	// Add message and fields
	builder.WriteString(e.Message)
	h.formatFields(&builder, e)

	// fmt.Println("------------>", len(e.Stack))
	// Format stack trace if present
	if len(e.Stack) > 0 {
		h.formatStack(&builder, e.Stack)
	}

	// Append newline for non-None levels
	if e.Level != lx.LevelNone {
		builder.WriteString(lx.Newline)
	}

	// Write formatted output to writer
	_, err := h.w.Write([]byte(builder.String()))
	return err
}

// formatNamespace formats the namespace with ANSI color codes.
// It supports FlatPath ([parent/child]) and NestedPath ([parent]→[child]) styles.
// Example (internal usage):
//
//	h.formatNamespace(&builder, &lx.Entry{Namespace: "parent/child", Style: lx.FlatPath}) // Writes "[parent/child]: "
func (h *ColorizedHandler) formatNamespace(b *strings.Builder, e *lx.Entry) {
	if e.Namespace == "" {
		return
	}

	b.WriteString(lx.LeftBracket)
	switch e.Style {
	case lx.NestedPath:
		// Split namespace and format as [parent]→[child]
		parts := strings.Split(e.Namespace, lx.Slash)
		for i, part := range parts {
			b.WriteString(part)
			b.WriteString(lx.RightBracket)
			if i < len(parts)-1 {
				b.WriteString(lx.Arrow)
				b.WriteString(lx.LeftBracket)
			}
		}
	default: // FlatPath
		// Format as [parent/child]
		b.WriteString(e.Namespace)
		b.WriteString(lx.RightBracket)
	}
	b.WriteString(lx.Colon)
	b.WriteString(lx.Space)
}

// formatLevel formats the log level with ANSI color codes.
// It applies a color based on the level (Debug, Info, Warn, Error) and resets afterward.
// Example (internal usage):
//
//	h.formatLevel(&builder, &lx.Entry{Level: lx.LevelInfo}) // Writes "<green>INFO<reset>: "
func (h *ColorizedHandler) formatLevel(b *strings.Builder, e *lx.Entry) {
	// Map levels to colors
	color := map[lx.LevelType]string{
		lx.LevelDebug: h.palette.Debug, // Cyan
		lx.LevelInfo:  h.palette.Info,  // Green
		lx.LevelWarn:  h.palette.Warn,  // Yellow
		lx.LevelError: h.palette.Error, // Red
	}[e.Level]

	b.WriteString(color)
	b.WriteString(e.Level.String())
	b.WriteString(h.palette.Reset)
	b.WriteString(lx.Colon)
	b.WriteString(lx.Space)
}

// formatFields formats the log entry's fields in sorted order.
// It writes fields as [key=value key=value], with no additional coloring.
// Example (internal usage):
//
//	h.formatFields(&builder, &lx.Entry{Fields: map[string]interface{}{"key": "value"}}) // Writes " [key=value]"
func (h *ColorizedHandler) formatFields(b *strings.Builder, e *lx.Entry) {
	if len(e.Fields) == 0 {
		return
	}

	// Collect and sort field keys
	var keys []string
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b.WriteString(lx.Space)
	b.WriteString(lx.LeftBracket)
	// Format fields as key=value
	for i, k := range keys {
		if i > 0 {
			b.WriteString(lx.Space)
		}
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(fmt.Sprint(e.Fields[k]))
	}
	b.WriteString(lx.RightBracket)
}

// formatStack formats a stack trace with ANSI color codes.
// It structures the stack trace with colored goroutine, function, and path segments,
// using indentation and separators for readability.
// Example (internal usage):
//
//	h.formatStack(&builder, []byte("goroutine 1 [running]:\nmain.main()\n\tmain.go:10")) // Appends colored stack trace
func (h *ColorizedHandler) formatStack(b *strings.Builder, stack []byte) {
	b.WriteString("\n")
	b.WriteString(h.palette.Header)
	b.WriteString("[stack]")
	b.WriteString(h.palette.Reset)
	b.WriteString("\n")

	lines := strings.Split(string(stack), "\n")
	if len(lines) == 0 {
		return
	}

	// Format goroutine line
	b.WriteString("  ┌─ ")
	b.WriteString(h.palette.Goroutine)
	b.WriteString(lines[0])
	b.WriteString(h.palette.Reset)
	b.WriteString("\n")

	// Pair function name and file path lines
	for i := 1; i < len(lines)-1; i += 2 {
		funcLine := strings.TrimSpace(lines[i])
		pathLine := strings.TrimSpace(lines[i+1])

		if funcLine != "" {
			b.WriteString("  │   ")
			b.WriteString(h.palette.Func)
			b.WriteString(funcLine)
			b.WriteString(h.palette.Reset)
			b.WriteString("\n")
		}
		if pathLine != "" {
			b.WriteString("  │   ")

			// Look for last "/" before ".go:"
			lastSlash := strings.LastIndex(pathLine, "/")
			goIndex := strings.Index(pathLine, ".go:")

			if lastSlash >= 0 && goIndex > lastSlash {
				// Prefix path
				prefix := pathLine[:lastSlash+1]
				// File and line (e.g., ll.go:698 +0x5c)
				suffix := pathLine[lastSlash+1:]

				b.WriteString(h.palette.Path)
				b.WriteString(prefix)
				b.WriteString(h.palette.Reset)

				b.WriteString(h.palette.Path) // Use mainPath color for suffix
				b.WriteString(suffix)
				b.WriteString(h.palette.Reset)
			} else {
				// Fallback: whole line is gray
				b.WriteString(h.palette.Path)
				b.WriteString(pathLine)
				b.WriteString(h.palette.Reset)
			}

			b.WriteString("\n")
		}
	}

	// Handle any remaining unpaired line
	if len(lines)%2 == 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		b.WriteString("  │   ")
		b.WriteString(h.palette.Func)
		b.WriteString(strings.TrimSpace(lines[len(lines)-1]))
		b.WriteString(h.palette.Reset)
		b.WriteString("\n")
	}

	b.WriteString("  └\n")
}

// handleDumpOutput formats hex dump output with ANSI color codes.
// It applies colors to position, hex, ASCII, and title components of the dump,
// wrapping the output with colored BEGIN/END separators.
// Returns an error if writing fails.
// Example (internal usage):
//
//	h.handleDumpOutput(&lx.Entry{Class: lx.ClassDump, Message: "pos 00 hex: 61 62 'ab'"}) // Writes colored dump
func (h *ColorizedHandler) handleDumpOutput(e *lx.Entry) error {
	var builder strings.Builder

	// Add timestamp if enabled
	if h.showTime {
		builder.WriteString(e.Timestamp.Format(h.timeFormat))
		builder.WriteString(lx.Newline)
	}

	// Write colored BEGIN separator
	builder.WriteString(h.palette.Title)
	builder.WriteString("---- BEGIN DUMP ----")
	builder.WriteString(h.palette.Reset)
	builder.WriteString("\n")

	// Process each line of the dump
	lines := strings.Split(e.Message, "\n")
	length := len(lines)
	for i, line := range lines {
		if strings.HasPrefix(line, "pos ") {
			// Parse and color position and hex/ASCII parts
			parts := strings.SplitN(line, "hex:", 2)
			if len(parts) == 2 {
				builder.WriteString(h.palette.Pos)
				builder.WriteString(parts[0])
				builder.WriteString(h.palette.Reset)

				hexAscii := strings.SplitN(parts[1], "'", 2)
				builder.WriteString(h.palette.Hex)
				builder.WriteString("hex:")
				builder.WriteString(hexAscii[0])
				builder.WriteString(h.palette.Reset)

				if len(hexAscii) > 1 {
					builder.WriteString(h.palette.Ascii)
					builder.WriteString("'")
					builder.WriteString(hexAscii[1])
					builder.WriteString(h.palette.Reset)
				}
			}
		} else if strings.HasPrefix(line, "Dumping value of type:") {
			// Color type dump lines
			builder.WriteString(h.palette.Header)
			builder.WriteString(line)
			builder.WriteString(h.palette.Reset)
		} else {
			// Write non-dump lines as-is
			builder.WriteString(line)
		}

		// Don't add newline for the last line
		if i < length-1 {
			builder.WriteString("\n")
		}
	}

	// Write colored END separator
	builder.WriteString(h.palette.Title)
	builder.WriteString("---- END DUMP ----")
	builder.WriteString(h.palette.Reset)
	builder.WriteString("\n")

	// Write formatted output to writer
	_, err := h.w.Write([]byte(builder.String()))
	return err
}

// detectPalette selects a color palette based on terminal environment variables.
// It checks TERM_BACKGROUND, COLORFGBG, and AppleInterfaceStyle to determine
// whether a light or dark palette is appropriate, defaulting to darkPalette.
// Example (internal usage):
//
//	palette := h.detectPalette() // Returns darkPalette or lightPalette
func (h *ColorizedHandler) detectPalette() Palette {
	// Check TERM_BACKGROUND (e.g., iTerm2)
	if bg, ok := os.LookupEnv("TERM_BACKGROUND"); ok {
		if bg == "light" {
			return lightPalette // Use light palette for light background
		}
		return darkPalette // Use dark palette otherwise
	}

	// Check COLORFGBG (traditional xterm)
	if fgBg, ok := os.LookupEnv("COLORFGBG"); ok {
		parts := strings.Split(fgBg, ";")
		if len(parts) >= 2 {
			bg := parts[len(parts)-1]                    // Last part (some terminals add more fields)
			if bg == "7" || bg == "15" || bg == "0;15" { // Handle variations
				return lightPalette // Use light palette for light background
			}
		}
	}

	// Check macOS dark mode
	if style, ok := os.LookupEnv("AppleInterfaceStyle"); ok && strings.EqualFold(style, "dark") {
		return darkPalette // Use dark palette for macOS dark mode
	}

	// Default: dark (conservative choice for terminals)
	return darkPalette
}
