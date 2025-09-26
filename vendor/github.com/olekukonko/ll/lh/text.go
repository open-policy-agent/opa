package lh

import (
	"fmt"
	"github.com/olekukonko/ll/lx"
	"io"
	"sort"
	"strings"
	"time"
)

// TextHandler is a handler that outputs log entries as plain text.
// It formats log entries with namespace, level, message, fields, and optional stack traces,
// writing the result to the provided writer.
// Thread-safe if the underlying writer is thread-safe.
type TextHandler struct {
	w          io.Writer // Destination for formatted log output
	showTime   bool      // Whether to display timestamps
	timeFormat string    // Format for timestamps (defaults to time.RFC3339)
}

// NewTextHandler creates a new TextHandler writing to the specified writer.
// It initializes the handler with the given writer, suitable for outputs like stdout or files.
// Example:
//
//	handler := NewTextHandler(os.Stdout)
//	logger := ll.New("app").Enable().Handler(handler)
//	logger.Info("Test") // Output: [app] INFO: Test
func NewTextHandler(w io.Writer) *TextHandler {
	return &TextHandler{
		w:          w,
		showTime:   false,
		timeFormat: time.RFC3339,
	}
}

// Timestamped enables or disables timestamp display and optionally sets a custom time format.
// If format is empty, defaults to RFC3339.
// Example:
//
//	handler := NewTextHandler(os.Stdout).TextWithTime(true, time.StampMilli)
//	// Output: Jan 02 15:04:05.000 [app] INFO: Test
func (h *TextHandler) Timestamped(enable bool, format ...string) {
	h.showTime = enable
	if len(format) > 0 && format[0] != "" {
		h.timeFormat = format[0]
	}
}

// Handle processes a log entry and writes it as plain text.
// It delegates to specialized methods based on the entry's class (Dump, Raw, or regular).
// Returns an error if writing to the underlying writer fails.
// Thread-safe if the writer is thread-safe.
// Example:
//
//	handler.Handle(&lx.Entry{Message: "test", Level: lx.LevelInfo}) // Writes "INFO: test"
func (h *TextHandler) Handle(e *lx.Entry) error {
	// Special handling for dump output
	if e.Class == lx.ClassDump {
		return h.handleDumpOutput(e)
	}

	// Raw entries are written directly without formatting
	if e.Class == lx.ClassRaw {
		_, err := h.w.Write([]byte(e.Message))
		return err
	}

	// Handle standard log entries
	return h.handleRegularOutput(e)
}

// handleRegularOutput handles normal log entries.
// It formats the entry with namespace, level, message, fields, and stack trace (if present),
// writing the result to the handler's writer.
// Returns an error if writing fails.
// Example (internal usage):
//
//	h.handleRegularOutput(&lx.Entry{Message: "test", Level: lx.LevelInfo}) // Writes "INFO: test"
func (h *TextHandler) handleRegularOutput(e *lx.Entry) error {
	var builder strings.Builder // Buffer for building formatted output

	// Add timestamp if enabled
	if h.showTime {
		builder.WriteString(e.Timestamp.Format(h.timeFormat))
		builder.WriteString(lx.Space)
	}

	// Format namespace based on style
	switch e.Style {
	case lx.NestedPath:
		if e.Namespace != "" {
			// Split namespace into parts and format as [parent]→[child]
			parts := strings.Split(e.Namespace, lx.Slash)
			for i, part := range parts {
				builder.WriteString(lx.LeftBracket)
				builder.WriteString(part)
				builder.WriteString(lx.RightBracket)
				if i < len(parts)-1 {
					builder.WriteString(lx.Arrow)
				}
			}
			builder.WriteString(lx.Colon)
			builder.WriteString(lx.Space)
		}
	default: // FlatPath
		if e.Namespace != "" {
			// Format namespace as [parent/child]
			builder.WriteString(lx.LeftBracket)
			builder.WriteString(e.Namespace)
			builder.WriteString(lx.RightBracket)
			builder.WriteString(lx.Space)
		}
	}

	// Add level and message
	builder.WriteString(e.Level.String())
	builder.WriteString(lx.Colon)
	builder.WriteString(lx.Space)
	builder.WriteString(e.Message)

	// Add fields in sorted order
	if len(e.Fields) > 0 {
		var keys []string
		for k := range e.Fields {
			keys = append(keys, k)
		}
		// Sort keys for consistent output
		sort.Strings(keys)
		builder.WriteString(lx.Space)
		builder.WriteString(lx.LeftBracket)
		for i, k := range keys {
			if i > 0 {
				builder.WriteString(lx.Space)
			}
			// Format field as key=value
			builder.WriteString(k)
			builder.WriteString("=")
			builder.WriteString(fmt.Sprint(e.Fields[k]))
		}
		builder.WriteString(lx.RightBracket)
	}

	// Add stack trace if present
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

// handleDumpOutput specially formats hex dump output (plain text version).
// It wraps the dump message with BEGIN/END separators for clarity.
// Returns an error if writing fails.
// Example (internal usage):
//
//	h.handleDumpOutput(&lx.Entry{Class: lx.ClassDump, Message: "pos 00 hex: 61"}) // Writes "---- BEGIN DUMP ----\npos 00 hex: 61\n---- END DUMP ----\n"
func (h *TextHandler) handleDumpOutput(e *lx.Entry) error {
	// For text handler, we just add a newline before dump output
	var builder strings.Builder // Buffer for building formatted output

	// Add timestamp if enabled
	if h.showTime {
		builder.WriteString(e.Timestamp.Format(h.timeFormat))
		builder.WriteString(lx.Newline)
	}

	// Add separator lines and dump content
	builder.WriteString("---- BEGIN DUMP ----\n")
	builder.WriteString(e.Message)
	builder.WriteString("---- END DUMP ----\n")

	// Write formatted output to writer
	_, err := h.w.Write([]byte(builder.String()))
	return err
}

// formatStack formats a stack trace for plain text output.
// It structures the stack trace with indentation and separators for readability,
// including goroutine and function/file details.
// Example (internal usage):
//
//	h.formatStack(&builder, []byte("goroutine 1 [running]:\nmain.main()\n\tmain.go:10")) // Appends formatted stack trace
func (h *TextHandler) formatStack(b *strings.Builder, stack []byte) {
	lines := strings.Split(string(stack), "\n")
	if len(lines) == 0 {
		return
	}

	// Start stack trace section
	b.WriteString("\n[stack]\n")

	// First line: goroutine
	b.WriteString("  ┌─ ")
	b.WriteString(lines[0])
	b.WriteString("\n")

	// Iterate through remaining lines
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		if strings.Contains(line, ".go") {
			// File path lines get extra indent
			b.WriteString("  ├       ")
		} else {
			// Function names
			b.WriteString("  │   ")
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	// End stack trace section
	b.WriteString("  └\n")
}
