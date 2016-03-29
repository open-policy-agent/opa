package builder

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/PuerkitoBio/pigeon/bootstrap"
)

var grammar = ` 
{
var test = "some string"

func init() {
	fmt.Println("this is inside the init")
}
}

start = additive eof
additive = left:multiplicative "+" space right:additive {
	fmt.Println(left, right)
} / mul:multiplicative { fmt.Println(mul) }
multiplicative = left:primary op:"*" space right:multiplicative { fmt.Println(left, right, op) } / primary
primary = integer / "(" space additive:additive ")" space { fmt.Println(additive) }
integer "integer" = digits:[0123456789]+ space { fmt.Println(digits) }
space = ' '*
eof = !. { fmt.Println("eof") }
`

func TestBuildParser(t *testing.T) {
	p := bootstrap.NewParser()
	g, err := p.Parse("", strings.NewReader(grammar))
	if err != nil {
		t.Fatal(err)
	}
	if err := BuildParser(ioutil.Discard, g); err != nil {
		t.Fatal(err)
	}
}
