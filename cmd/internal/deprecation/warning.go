package deprecation

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

const width, border = 80, 3
const titleChar, dividerChar = "#", "-"

// warning is a struct which can be used to define deprecation warnings based on
// the environment and command being run.
type warning struct {
	MatchEnv     func([]string) bool
	MatchCommand func(string) bool
	Fatal        bool
	Message      string
}

// CheckWarnings runs messageForWarnings with a default set of real warnings.
func CheckWarnings(env []string, command string) (string, bool) {
	warnings := []warning{
		warningRootless,
		warningRootlessRun,
	}

	return messageForWarnings(warnings, env, command)
}

// messageForWarnings returns an obnoxious banner with the contents of all firing warnings.
// If no warnings fire, it returns an empty string.
// If any warnings are fatal, it returns true for the second return value.
func messageForWarnings(warnings []warning, env []string, command string) (string, bool) {
	var messages []string
	var fatal bool

	for _, w := range warnings {
		if w.MatchEnv(env) && w.MatchCommand(command) {
			messages = append(messages, w.Message)
			if w.Fatal {
				fatal = true
			}
		}
	}

	buf := bytes.NewBuffer(nil)

	if len(messages) == 0 {
		return "", false
	}

	title := "Deprecation Warnings"
	if fatal {
		title = "Fatal Deprecation Warnings"
	}

	printFormattedTitle(buf, title)

	for i, msg := range messages {
		fmt.Fprintln(buf, strings.TrimSpace(msg))
		if i < len(messages)-1 {
			printFormattedDivider(buf)
		}
	}

	printFormattedTitle(buf, "end "+title)

	return buf.String(), fatal
}

func printFormattedTitle(out io.Writer, title string) {
	padding := (width - len(title) - border*2) / 2

	fmt.Fprintln(out, strings.Repeat(titleChar, width))
	fmt.Fprintln(out,
		strings.Join(
			[]string{
				strings.Repeat(titleChar, border),
				strings.Repeat(" ", padding), strings.ToUpper(title), strings.Repeat(" ", padding),
				strings.Repeat(titleChar, border),
			},
			"",
		),
	)
	fmt.Fprintln(out, strings.Repeat(titleChar, width))
}

func printFormattedDivider(out io.Writer) {
	fmt.Fprintln(out, strings.Repeat(dividerChar, width))
}
