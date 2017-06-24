package bootstrap

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"unicode"

	"github.com/mna/pigeon/ast"
)

// Scanner tokenizes an input source for the PEG grammar.
type Scanner struct {
	r    io.RuneReader
	errh func(ast.Pos, error)

	eof  bool
	cur  rune
	cpos ast.Pos
	cw   int

	tok bytes.Buffer
}

// Init initializes the scanner to read and tokenize text from r.
func (s *Scanner) Init(filename string, r io.Reader, errh func(ast.Pos, error)) {
	s.r = runeReader(r)
	s.errh = errh

	s.eof = false
	s.cpos = ast.Pos{
		Filename: filename,
		Line:     1,
	}

	s.cur, s.cw = -1, 0
	s.tok.Reset()
}

// Scan returns the next token, along with a boolean indicating if EOF was
// reached (false means no more tokens).
func (s *Scanner) Scan() (Token, bool) {
	var tok Token

	if !s.eof && s.cur == -1 {
		// move to first rune
		s.read()
	}

	s.skipWhitespace()
	tok.pos = s.cpos

	// the first switch cases all position the scanner on the next rune
	// by their calls to scan*
	switch {
	case s.eof:
		tok.id = eof
	case isLetter(s.cur):
		tok.id = ident
		tok.lit = s.scanIdentifier()
		if _, ok := blacklistedIdents[tok.lit]; ok {
			s.errorpf(tok.pos, "illegal identifier %q", tok.lit)
		}
	case isRuleDefStart(s.cur):
		tok.id = ruledef
		tok.lit = s.scanRuleDef()
	case s.cur == '\'':
		tok.id = char
		tok.lit = s.scanChar()
	case s.cur == '"':
		tok.id = str
		tok.lit = s.scanString()
	case s.cur == '`':
		tok.id = rstr
		tok.lit = s.scanRawString()
	case s.cur == '[':
		tok.id = class
		tok.lit = s.scanClass()
	case s.cur == '{':
		tok.id = code
		tok.lit = s.scanCode()

	default:
		r := s.cur
		s.read()
		switch r {
		case '/':
			if s.cur == '*' || s.cur == '/' {
				tok.id, tok.lit = s.scanComment()
				break
			}
			fallthrough
		case ':', ';', '(', ')', '.', '&', '!', '?', '+', '*', '\n':
			tok.id = tid(r)
			tok.lit = string(r)
		default:
			s.errorf("invalid character %#U", r)
			tok.id = invalid
			tok.lit = string(r)
		}
	}

	return tok, tok.id != eof
}

func (s *Scanner) scanIdentifier() string {
	s.tok.Reset()
	for isLetter(s.cur) || isDigit(s.cur) {
		s.tok.WriteRune(s.cur)
		s.read()
	}
	return s.tok.String()
}

func (s *Scanner) scanComment() (tid, string) {
	s.tok.Reset()
	s.tok.WriteRune('/') // initial '/' already consumed

	var multiline bool
	switch s.cur {
	case '*':
		multiline = true
	case '\n', -1:
		s.errorf("comment not terminated")
		return lcomment, s.tok.String()
	}

	var closing bool
	for {
		s.tok.WriteRune(s.cur)
		s.read()
		switch s.cur {
		case '\n':
			if !multiline {
				return lcomment, s.tok.String()
			}
		case -1:
			if multiline {
				s.errorf("comment not terminated")
				return mlcomment, s.tok.String()
			}
			return lcomment, s.tok.String()
		case '*':
			if multiline {
				closing = true
			}
		case '/':
			if closing {
				s.tok.WriteRune(s.cur)
				s.read()
				return mlcomment, s.tok.String()
			}
		}
	}
}

func (s *Scanner) scanCode() string {
	s.tok.Reset()
	s.tok.WriteRune(s.cur)
	depth := 1
	for {
		s.read()
		s.tok.WriteRune(s.cur)
		switch s.cur {
		case -1:
			s.errorf("code block not terminated")
			return s.tok.String()
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				s.read()
				return s.tok.String()
			}
		}
	}
}

func (s *Scanner) scanEscape(quote rune) bool {
	// scanEscape is always called as part of a greater token, so do not
	// reset s.tok, and write s.cur before calling s.read.
	s.tok.WriteRune(s.cur)

	var n int
	var base, max uint32
	var unicodeClass bool

	s.read()
	switch s.cur {
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', quote:
		s.tok.WriteRune(s.cur)
		return true
	case '0', '1', '2', '3', '4', '5', '6', '7':
		n, base, max = 3, 8, 255
	case 'x':
		s.tok.WriteRune(s.cur)
		s.read()
		n, base, max = 2, 16, 255
	case 'u':
		s.tok.WriteRune(s.cur)
		s.read()
		n, base, max = 4, 16, unicode.MaxRune
	case 'U':
		s.tok.WriteRune(s.cur)
		s.read()
		n, base, max = 8, 16, unicode.MaxRune
	case 'p':
		// unicode character class, only valid if quote is ']'
		if quote == ']' {
			s.tok.WriteRune(s.cur)
			unicodeClass = true
			s.read()
			break
		}
		fallthrough
	default:
		s.tok.WriteRune(s.cur)
		msg := "unknown escape sequence"
		if s.cur == -1 || s.cur == '\n' {
			msg = "escape sequence not terminated"
			s.errorf(msg)
		} else {
			s.errorf(msg)
			s.read()
		}
		return false
	}

	if unicodeClass {
		switch s.cur {
		case '\n', -1:
			s.errorf("escape sequence not terminated")
			return false
		case '{':
			// unicode class name, read until '}'
			cnt := 0
			for {
				s.tok.WriteRune(s.cur)
				s.read()
				cnt++
				switch s.cur {
				case '\n', -1:
					s.errorf("escape sequence not terminated")
					return false
				case '}':
					if cnt < 2 {
						s.errorf("empty Unicode character class escape sequence")
					}
					s.tok.WriteRune(s.cur)
					return true
				}
			}
		default:
			// single letter class
			s.tok.WriteRune(s.cur)
			return true
		}
	}

	var x uint32
	for n > 0 {
		s.tok.WriteRune(s.cur)
		d := uint32(digitVal(s.cur))
		if d >= base {
			msg := fmt.Sprintf("illegal character %#U in escape sequence", s.cur)
			if s.cur == -1 || s.cur == '\n' {
				msg = "escape sequence not terminated"
				s.errorf(msg)
				return false
			}
			s.errorf(msg)
			s.read()
			return false
		}
		x = x*base + d
		n--

		if n > 0 {
			s.read()
		}
	}

	if x > max || 0xd800 <= x && x <= 0xe000 {
		s.errorf("escape sequence is invalid Unicode code point")
		s.read()
		return false
	}
	return true
}

func (s *Scanner) scanClass() string {
	s.tok.Reset()
	s.tok.WriteRune(s.cur) // opening '['

	var noread bool
	for {
		if !noread {
			s.read()
		}
		noread = false
		switch s.cur {
		case '\\':
			noread = !s.scanEscape(']')
		case '\n', -1:
			// \n not consumed
			s.errorf("character class not terminated")
			return s.tok.String()
		case ']':
			s.tok.WriteRune(s.cur)
			s.read()
			// can have an optional "i" ignore case suffix
			if s.cur == 'i' {
				s.tok.WriteRune(s.cur)
				s.read()
			}
			return s.tok.String()
		default:
			s.tok.WriteRune(s.cur)
		}
	}
}

func (s *Scanner) scanRawString() string {
	s.tok.Reset()
	s.tok.WriteRune(s.cur) // opening '`'

	var hasCR bool
loop:
	for {
		s.read()
		switch s.cur {
		case -1:
			s.errorf("raw string literal not terminated")
			break loop
		case '`':
			s.tok.WriteRune(s.cur)
			s.read()
			// can have an optional "i" ignore case suffix
			if s.cur == 'i' {
				s.tok.WriteRune(s.cur)
				s.read()
			}
			break loop
		case '\r':
			hasCR = true
			fallthrough
		default:
			s.tok.WriteRune(s.cur)
		}
	}

	b := s.tok.Bytes()
	if hasCR {
		b = stripCR(b)
	}
	return string(b)
}

func stripCR(b []byte) []byte {
	c := make([]byte, len(b))
	i := 0
	for _, ch := range b {
		if ch != '\r' {
			c[i] = ch
			i++
		}
	}
	return c[:i]
}

func (s *Scanner) scanString() string {
	s.tok.Reset()
	s.tok.WriteRune(s.cur) // opening '"'

	var noread bool
	for {
		if !noread {
			s.read()
		}
		noread = false
		switch s.cur {
		case '\\':
			noread = !s.scanEscape('"')
		case '\n', -1:
			// \n not consumed
			s.errorf("string literal not terminated")
			return s.tok.String()
		case '"':
			s.tok.WriteRune(s.cur)
			s.read()
			// can have an optional "i" ignore case suffix
			if s.cur == 'i' {
				s.tok.WriteRune(s.cur)
				s.read()
			}
			return s.tok.String()
		default:
			s.tok.WriteRune(s.cur)
		}
	}
}

func (s *Scanner) scanChar() string {
	s.tok.Reset()
	s.tok.WriteRune(s.cur) // opening "'"

	// must be followed by one char (which may be an escape) and a single
	// quote, but read until we find that closing quote.
	cnt := 0
	var noread bool
	for {
		if !noread {
			s.read()
		}
		noread = false
		switch s.cur {
		case '\\':
			cnt++
			noread = !s.scanEscape('\'')
		case '\n', -1:
			// \n not consumed
			s.errorf("rune literal not terminated")
			return s.tok.String()
		case '\'':
			s.tok.WriteRune(s.cur)
			s.read()
			if cnt != 1 {
				s.errorf("rune literal is not a single rune")
			}
			// can have an optional "i" ignore case suffix
			if s.cur == 'i' {
				s.tok.WriteRune(s.cur)
				s.read()
			}
			return s.tok.String()
		default:
			cnt++
			s.tok.WriteRune(s.cur)
		}
	}
}

func (s *Scanner) scanRuleDef() string {
	s.tok.Reset()
	s.tok.WriteRune(s.cur)
	r := s.cur
	s.read()
	if r == '<' {
		if s.cur != -1 {
			s.tok.WriteRune(s.cur)
		}
		if s.cur != '-' {
			s.errorf("rule definition not terminated")
		}
		s.read()
	}

	return s.tok.String()
}

// read advances the Scanner to the next rune.
func (s *Scanner) read() {
	if s.eof {
		return
	}

	r, w, err := s.r.ReadRune()
	if err != nil {
		s.fatalError(err)
		return
	}

	s.cur = r
	s.cpos.Off += s.cw
	s.cw = w

	// newline is '\n' as in Go
	if r == '\n' {
		s.cpos.Line++
		s.cpos.Col = 0
	} else {
		s.cpos.Col++
	}
}

// whitespace is the same as Go, except that it doesn't skip newlines,
// those are returned as tokens.
func (s *Scanner) skipWhitespace() {
	for s.cur == ' ' || s.cur == '\t' || s.cur == '\r' {
		s.read()
	}
}

func isRuleDefStart(r rune) bool {
	return r == '=' || r == '<' || r == '\u2190' /* leftwards arrow */ ||
		r == '\u27f5' /* long leftwards arrow */
}

// isLetter has the same definition as Go.
func isLetter(r rune) bool {
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || r == '_' ||
		r >= 0x80 && unicode.IsLetter(r)
}

// isDigit has the same definition as Go.
func isDigit(r rune) bool {
	return '0' <= r && r <= '9' || r >= 0x80 && unicode.IsDigit(r)
}

func digitVal(r rune) int {
	switch {
	case '0' <= r && r <= '9':
		return int(r - '0')
	case 'a' <= r && r <= 'f':
		return int(r - 'a' + 10)
	case 'A' <= r && r <= 'F':
		return int(r - 'A' + 10)
	}
	return 16
}

// notify the handler of an error.
func (s *Scanner) error(p ast.Pos, err error) {
	if s.errh != nil {
		s.errh(p, err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
}

// helper to generate and notify of an error.
func (s *Scanner) errorf(f string, args ...interface{}) {
	s.errorpf(s.cpos, f, args...)
}

// helper to generate and notify of an error at a specific position.
func (s *Scanner) errorpf(p ast.Pos, f string, args ...interface{}) {
	s.error(p, fmt.Errorf(f, args...))
}

// notify a non-recoverable error that terminates the scanning.
func (s *Scanner) fatalError(err error) {
	s.cur = -1
	s.eof = true
	if err != io.EOF {
		s.error(s.cpos, err)
	}
}

// convert the reader to a rune reader if required.
func runeReader(r io.Reader) io.RuneReader {
	if rr, ok := r.(io.RuneReader); ok {
		return rr
	}
	return bufio.NewReader(r)
}
