package exec

import (
	"bufio"
	"io"
	"strings"
)

type stdInReader struct {
	Reader io.Reader
}

func (sr *stdInReader) ReadInput() string {
	var lines []string
	in := bufio.NewScanner(sr.Reader)
	for {
		in.Scan()
		line := in.Text()
		if len(line) == 0 {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
