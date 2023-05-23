package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra/doc"

	"github.com/open-policy-agent/opa/cmd"
)

const fileHeader = `---
title: CLI
kind: documentation
weight: 90
restrictedtoc: true
---

The OPA executable provides the following commands.

`

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Required argument: cli docs output directory")
	}
	out := os.Args[1]

	command := cmd.RootCommand
	command.Use = "opa [command]"
	command.DisableAutoGenTag = true

	dir, err := os.MkdirTemp("", "opa")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	err = doc.GenMarkdownTree(command, dir)
	if err != nil {
		log.Fatal(err)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	builder := strings.Builder{}

	last := len(files) - 1
	for i, file := range files {
		// Skip the first "opa" document as it's rather pointless to include (only shows the --help flag)
		if i == 0 {
			continue
		}
		path := filepath.Join(dir, file.Name())
		document, err := fixupSection(path)
		if err != nil {
			log.Fatal(err)
		}
		builder.WriteString(document)
		if i != last {
			builder.WriteString("____\n\n")
		}
	}

	heading := regexp.MustCompile(`^[\\-]+$`)
	lines := strings.Split(builder.String(), "\n")
	document := make([]string, 0, len(lines))
	removed := 0

	// The document may contain "----" for headings, which will be converted to h1
	// elements in markdown. This is undesirable, so let's remove them and prepend
	// the line before that with ### to instead create a h3
	for line, str := range lines {
		if heading.MatchString(str) {
			document[line-1-removed] = fmt.Sprintf("### %s\n", document[line-1-removed])
			removed++
			continue
		}
		document = append(document, fmt.Sprintf("%s\n", str))
	}

	withHeader := fmt.Sprintf("%s%s", fileHeader, strings.Join(document, ""))
	err = os.WriteFile(filepath.Join(out, "cli.md"), []byte(withHeader), 0755)
	if err != nil {
		log.Fatal(err)
	}
}

func fixupSection(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	builder := strings.Builder{}

	for scanner.Scan() {
		line := scanner.Text()
		// Remove "See also" section
		if strings.Contains(line, "### SEE ALSO") {
			break
		}
		if home := os.Getenv("HOME"); home != "" {
			line = strings.ReplaceAll(line, home, "$HOME")
		}
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	return builder.String(), scanner.Err()
}
