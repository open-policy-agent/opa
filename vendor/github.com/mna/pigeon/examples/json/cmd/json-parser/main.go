package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/mna/pigeon/examples/json"
)

func main() {
	in := os.Stdin
	nm := "stdin"
	if len(os.Args) > 1 {
		f, err := os.Open(os.Args[1])
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			err := f.Close()
			if err != nil {
				log.Fatal(err)
			}
		}()
		in = f
		nm = os.Args[1]
	}

	b, err := ioutil.ReadAll(in)
	if err != nil {
		log.Fatal(err)
	}

	got, err := json.Parse(nm, b)
	if err != nil {
		fmt.Println(caretError(err, string(b)))
		os.Exit(1)
	}
	fmt.Println(got)
}

func caretError(err error, input string) string {
	if el, ok := err.(json.ErrorLister); ok {
		var buffer bytes.Buffer
		for _, e := range el.Errors() {
			if parserErr, ok := e.(json.ParserError); ok {
				_, col, off := parserErr.Pos()
				line := extractLine(input, off)
				if col >= len(line) {
					col = len(line) - 1
				} else {
					if col > 0 {
						col--
					}
				}
				if col < 0 {
					col = 0
				}
				pos := col
				for _, chr := range line[:col] {
					if chr == '\t' {
						pos += 7
					}
				}
				buffer.WriteString(fmt.Sprintf("%s\n%s\n%s\n", line, strings.Repeat(" ", pos)+"^", err.Error()))
			} else {
				return err.Error()
			}
		}
		return buffer.String()
	}
	return err.Error()
}

func extractLine(input string, initPos int) string {
	if initPos < 0 {
		initPos = 0
	}
	if initPos >= len(input) && len(input) > 0 {
		initPos = len(input) - 1
	}
	startPos := initPos
	endPos := initPos
	for ; startPos > 0; startPos-- {
		if input[startPos] == '\n' {
			if startPos != initPos {
				startPos++
				break
			}
		}
	}
	for ; endPos < len(input); endPos++ {
		if input[endPos] == '\n' {
			if endPos == initPos {
				endPos++
			}
			break
		}
	}
	return input[startPos:endPos]
}
