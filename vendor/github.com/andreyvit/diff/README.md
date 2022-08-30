# diff

Quick'n'easy string diffing functions for Golang based on [github.com/sergi/go-diff](https://github.com/sergi/go-diff). Mainly for diffing strings in tests.

See [the docs on GoDoc](https://godoc.org/github.com/andreyvit/diff).

Get it:

    go get -u github.com/andreyvit/diff

Example:

    import (
        "strings"
        "testing"
        "github.com/andreyvit/diff"
    )

    const expected = `
    ...
    `

    func TestFoo(t *testing.T) {
        actual := Foo(...)
        if a, e := strings.TrimSpace(actual), strings.TrimSpace(expected); a != e {
            t.Errorf("Result not as expected:\n%v", diff.LineDiff(e, a))
        }
    }
