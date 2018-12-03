// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package imports

import (
	"fmt"
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var tests = []struct {
	name       string
	formatOnly bool
	in, out    string
}{
	// Adding an import to an existing parenthesized import
	{
		name: "factored_imports_add",
		in: `package foo
import (
  "fmt"
)
func bar() {
var b bytes.Buffer
fmt.Println(b.String())
}
`,
		out: `package foo

import (
	"bytes"
	"fmt"
)

func bar() {
	var b bytes.Buffer
	fmt.Println(b.String())
}
`,
	},

	// Adding an import to an existing parenthesized import,
	// verifying it goes into the first section.
	{
		name: "factored_imports_add_first_sec",
		in: `package foo
import (
  "fmt"

  "appengine"
)
func bar() {
var b bytes.Buffer
_ = appengine.IsDevServer
fmt.Println(b.String())
}
`,
		out: `package foo

import (
	"bytes"
	"fmt"

	"appengine"
)

func bar() {
	var b bytes.Buffer
	_ = appengine.IsDevServer
	fmt.Println(b.String())
}
`,
	},

	// Adding an import to an existing parenthesized import,
	// verifying it goes into the first section. (test 2)
	{
		name: "factored_imports_add_first_sec_2",
		in: `package foo
import (
  "fmt"

  "appengine"
)
func bar() {
_ = math.NaN
_ = fmt.Sprintf
_ = appengine.IsDevServer
}
`,
		out: `package foo

import (
	"fmt"
	"math"

	"appengine"
)

func bar() {
	_ = math.NaN
	_ = fmt.Sprintf
	_ = appengine.IsDevServer
}
`,
	},

	// Adding a new import line, without parens
	{
		name: "add_import_section",
		in: `package foo
func bar() {
var b bytes.Buffer
}
`,
		out: `package foo

import "bytes"

func bar() {
	var b bytes.Buffer
}
`,
	},

	// Adding two new imports, which should make a parenthesized import decl.
	{
		name: "add_import_paren_section",
		in: `package foo
func bar() {
_, _ := bytes.Buffer, zip.NewReader
}
`,
		out: `package foo

import (
	"archive/zip"
	"bytes"
)

func bar() {
	_, _ := bytes.Buffer, zip.NewReader
}
`,
	},

	// Make sure we don't add things twice
	{
		name: "no_double_add",
		in: `package foo
func bar() {
_, _ := bytes.Buffer, bytes.NewReader
}
`,
		out: `package foo

import "bytes"

func bar() {
	_, _ := bytes.Buffer, bytes.NewReader
}
`,
	},

	// Make sure we don't add packages that don't have the right exports
	{
		name: "no_mismatched_add",
		in: `package foo

func bar() {
	_ := bytes.NonexistentSymbol
}
`,
		out: `package foo

func bar() {
	_ := bytes.NonexistentSymbol
}
`,
	},

	// Remove unused imports, 1 of a factored block
	{
		name: "remove_unused_1_of_2",
		in: `package foo
import (
"bytes"
"fmt"
)

func bar() {
_, _ := bytes.Buffer, bytes.NewReader
}
`,
		out: `package foo

import (
	"bytes"
)

func bar() {
	_, _ := bytes.Buffer, bytes.NewReader
}
`,
	},

	// Remove unused imports, 2 of 2
	{
		name: "remove_unused_2_of_2",
		in: `package foo
import (
"bytes"
"fmt"
)

func bar() {
}
`,
		out: `package foo

func bar() {
}
`,
	},

	// Remove unused imports, 1 of 1
	{
		name: "remove_unused_1_of_1",
		in: `package foo

import "fmt"

func bar() {
}
`,
		out: `package foo

func bar() {
}
`,
	},

	// Don't remove empty imports.
	{
		name: "dont_remove_empty_imports",
		in: `package foo
import (
_ "image/png"
_ "image/jpeg"
)
`,
		out: `package foo

import (
	_ "image/jpeg"
	_ "image/png"
)
`,
	},

	// Don't remove dot imports.
	{
		name: "dont_remove_dot_imports",
		in: `package foo
import (
. "foo"
. "bar"
)
`,
		out: `package foo

import (
	. "bar"
	. "foo"
)
`,
	},

	// Skip refs the parser can resolve.
	{
		name: "skip_resolved_refs",
		in: `package foo

func f() {
	type t struct{ Println func(string) }
	fmt := t{Println: func(string) {}}
	fmt.Println("foo")
}
`,
		out: `package foo

func f() {
	type t struct{ Println func(string) }
	fmt := t{Println: func(string) {}}
	fmt.Println("foo")
}
`,
	},

	// Do not add a package we already have a resolution for.
	{
		name: "skip_template",
		in: `package foo

import "html/template"

func f() { t = template.New("sometemplate") }
`,
		out: `package foo

import "html/template"

func f() { t = template.New("sometemplate") }
`,
	},

	// Don't touch cgo
	{
		name: "cgo",
		in: `package foo

/*
#include <foo.h>
*/
import "C"
`,
		out: `package foo

/*
#include <foo.h>
*/
import "C"
`,
	},

	// Put some things in their own section
	{
		name: "make_sections",
		in: `package foo

import (
"os"
)

func foo () {
_, _ = os.Args, fmt.Println
_, _ = appengine.Main, datastore.ErrInvalidEntityType
}
`,
		out: `package foo

import (
	"fmt"
	"os"

	"appengine"
	"appengine/datastore"
)

func foo() {
	_, _ = os.Args, fmt.Println
	_, _ = appengine.Main, datastore.ErrInvalidEntityType
}
`,
	},

	// Delete existing empty import block
	{
		name: "delete_empty_import_block",
		in: `package foo

import ()
`,
		out: `package foo
`,
	},

	// Use existing empty import block
	{
		name: "use_empty_import_block",
		in: `package foo

import ()

func f() {
	_ = fmt.Println
}
`,
		out: `package foo

import "fmt"

func f() {
	_ = fmt.Println
}
`,
	},

	// Blank line before adding new section.
	{
		name: "blank_line_before_new_group",
		in: `package foo

import (
	"fmt"
	"net"
)

func f() {
	_ = net.Dial
	_ = fmt.Printf
	_ = snappy.ErrCorrupt
}
`,
		out: `package foo

import (
	"fmt"
	"net"

	"code.google.com/p/snappy-go/snappy"
)

func f() {
	_ = net.Dial
	_ = fmt.Printf
	_ = snappy.ErrCorrupt
}
`,
	},

	// Blank line between standard library and third-party stuff.
	{
		name: "blank_line_separating_std_and_third_party",
		in: `package foo

import (
	"code.google.com/p/snappy-go/snappy"
	"fmt"
	"net"
)

func f() {
	_ = net.Dial
	_ = fmt.Printf
	_ = snappy.Foo
}
`,
		out: `package foo

import (
	"fmt"
	"net"

	"code.google.com/p/snappy-go/snappy"
)

func f() {
	_ = net.Dial
	_ = fmt.Printf
	_ = snappy.Foo
}
`,
	},

	// golang.org/issue/6884
	{
		name: "new_imports_before_comment",
		in: `package main

// A comment
func main() {
	fmt.Println("Hello, world")
}
`,
		out: `package main

import "fmt"

// A comment
func main() {
	fmt.Println("Hello, world")
}
`,
	},

	// golang.org/issue/7132
	{
		name: "new_section_for_dotless_import",
		in: `package main

import (
"fmt"

"gu"
"github.com/foo/bar"
)

var (
a = bar.a
b = gu.a
c = fmt.Printf
)
`,
		out: `package main

import (
	"fmt"

	"gu"

	"github.com/foo/bar"
)

var (
	a = bar.a
	b = gu.a
	c = fmt.Printf
)
`,
	},

	{
		name: "fragment_with_main",
		in:   `func main(){fmt.Println("Hello, world")}`,
		out: `package main

import "fmt"

func main() { fmt.Println("Hello, world") }
`,
	},

	{
		name: "fragment_without_main",
		in:   `func notmain(){fmt.Println("Hello, world")}`,
		out: `import "fmt"

func notmain() { fmt.Println("Hello, world") }`,
	},

	// Remove first import within in a 2nd/3rd/4th/etc. section.
	// golang.org/issue/7679
	{
		name: "remove_first_import_in_section",
		in: `package main

import (
	"fmt"

	"github.com/foo/bar"
	"github.com/foo/qux"
)

func main() {
	var _ = fmt.Println
	//var _ = bar.A
	var _ = qux.B
}
`,
		out: `package main

import (
	"fmt"

	"github.com/foo/qux"
)

func main() {
	var _ = fmt.Println
	//var _ = bar.A
	var _ = qux.B
}
`,
	},

	// Blank line can be added before all types of import declarations.
	// golang.org/issue/7866
	{
		name: "new_section_for_all_kinds_of_imports",
		in: `package main

import (
	"fmt"
	renamed_bar "github.com/foo/bar"

	. "github.com/foo/baz"
	"io"

	_ "github.com/foo/qux"
	"strings"
)

var _, _, _, _, _ = fmt.Errorf, io.Copy, strings.Contains, renamed_bar.A, B
`,
		out: `package main

import (
	"fmt"

	renamed_bar "github.com/foo/bar"

	"io"

	. "github.com/foo/baz"

	"strings"

	_ "github.com/foo/qux"
)

var _, _, _, _, _ = fmt.Errorf, io.Copy, strings.Contains, renamed_bar.A, B
`,
	},

	// Non-idempotent comment formatting
	// golang.org/issue/8035
	{
		name: "comments_formatted",
		in: `package main

import (
	"fmt"                     // A
	"go/ast"                  // B
	_ "launchpad.net/gocheck" // C
)

func main() { _, _ = fmt.Print, ast.Walk }
`,
		out: `package main

import (
	"fmt"    // A
	"go/ast" // B

	_ "launchpad.net/gocheck" // C
)

func main() { _, _ = fmt.Print, ast.Walk }
`,
	},

	// Failure to delete all duplicate imports
	// golang.org/issue/8459
	{
		name: "remove_duplicates",
		in: `package main

import (
	"fmt"
	"log"
	"log"
	"math"
)

func main() { fmt.Println("pi:", math.Pi) }
`,
		out: `package main

import (
	"fmt"
	"math"
)

func main() { fmt.Println("pi:", math.Pi) }
`,
	},

	// Too aggressive prefix matching
	// golang.org/issue/9961
	{
		name: "no_extra_groups",
		in: `package p

import (
	"zip"

	"rsc.io/p"
)

var (
	_ = fmt.Print
	_ = zip.Store
	_ p.P
	_ = regexp.Compile
)
`,
		out: `package p

import (
	"fmt"
	"regexp"
	"zip"

	"rsc.io/p"
)

var (
	_ = fmt.Print
	_ = zip.Store
	_ p.P
	_ = regexp.Compile
)
`,
	},

	// Unused named import is mistaken for unnamed import
	// golang.org/issue/8149
	{
		name: "named_import_doesnt_provide_package_name",
		in: `package main

import foo "fmt"

func main() { fmt.Println() }
`,
		out: `package main

import "fmt"

func main() { fmt.Println() }
`,
	},

	// Unused named import is mistaken for unnamed import
	// golang.org/issue/8149
	{
		name: "unused_named_import_removed",
		in: `package main

import (
	"fmt"
	x "fmt"
)

func main() { fmt.Println() }
`,
		out: `package main

import (
	"fmt"
)

func main() { fmt.Println() }
`,
	},

	// FormatOnly
	{
		name:       "formatonly_works",
		formatOnly: true,
		in: `package main

import (
"fmt"
"golang.org/x/foo"
)

func main() {}
`,
		out: `package main

import (
	"fmt"

	"golang.org/x/foo"
)

func main() {}
`,
	},

	{
		name: "preserve_import_group",
		in: `package p

import (
	"bytes"
	"fmt"
)

var _ = fmt.Sprintf
`,
		out: `package p

import (
	"fmt"
)

var _ = fmt.Sprintf
`,
	},

	{
		name: "import_grouping_not_path_dependent_no_groups",
		in: `package main

import (
	"time"
)

func main() {
	_ = snappy.ErrCorrupt
	_ = p.P
	_ = time.Parse
}
`,
		out: `package main

import (
	"time"

	"code.google.com/p/snappy-go/snappy"
	"rsc.io/p"
)

func main() {
	_ = snappy.ErrCorrupt
	_ = p.P
	_ = time.Parse
}
`,
	},

	{
		name: "import_grouping_not_path_dependent_existing_group",
		in: `package main

import (
	"time"

	"code.google.com/p/snappy-go/snappy"
)

func main() {
	_ = snappy.ErrCorrupt
	_ = p.P
	_ = time.Parse
}
`,
		out: `package main

import (
	"time"

	"code.google.com/p/snappy-go/snappy"
	"rsc.io/p"
)

func main() {
	_ = snappy.ErrCorrupt
	_ = p.P
	_ = time.Parse
}
`,
	},

	{
		name: "issue #12097",
		in: `// a
// b
// c

func main() {
    _ = fmt.Println
}`,
		out: `package main

import "fmt"

// a
// b
// c

func main() {
	_ = fmt.Println
}
`,
	},

	{
		name: "import_comment_stays_on_import",
		in: `package main

import (
	"math" // fun
)

func main() {
	x := math.MaxInt64
	fmt.Println(strings.Join(",", []string{"hi"}), x)
}`,
		out: `package main

import (
	"fmt"
	"math" // fun
	"strings"
)

func main() {
	x := math.MaxInt64
	fmt.Println(strings.Join(",", []string{"hi"}), x)
}
`,
	},

	{
		name: "no_blank_after_comment",
		in: `package main

import (
	_ "io"
	_ "net/http"
	_ "net/http/pprof" // install the pprof http handlers
	_ "strings"
)

func main() {
}
`,
		out: `package main

import (
	_ "io"
	_ "net/http"
	_ "net/http/pprof" // install the pprof http handlers
	_ "strings"
)

func main() {
}
`,
	},

	{
		name: "no_blank_after_comment_reordered",
		in: `package main

import (
	_ "io"
	_ "net/http/pprof" // install the pprof http handlers
	_ "net/http"
	_ "strings"
)

func main() {
}
`,
		out: `package main

import (
	_ "io"
	_ "net/http"
	_ "net/http/pprof" // install the pprof http handlers
	_ "strings"
)

func main() {
}
`,
	},

	{
		name: "no_blank_after_comment_unnamed",
		in: `package main

import (
	"encoding/json"
	"io"
	"net/http"
	_ "net/http/pprof" // install the pprof http handlers
	"strings"

	"github.com/pkg/errors"
)

func main() {
	_ = strings.ToUpper("hello")
	_ = io.EOF
	var (
		_ json.Number
		_ *http.Request
		_ errors.Frame
	)
}
`,
		out: `package main

import (
	"encoding/json"
	"io"
	"net/http"
	_ "net/http/pprof" // install the pprof http handlers
	"strings"

	"github.com/pkg/errors"
)

func main() {
	_ = strings.ToUpper("hello")
	_ = io.EOF
	var (
		_ json.Number
		_ *http.Request
		_ errors.Frame
	)
}
`,
	},

	{
		name: "blank_after_package_statement_with_comment",
		in: `package p // comment

import "math"

var _ = fmt.Printf
`,
		out: `package p // comment

import "fmt"

var _ = fmt.Printf
`,
	},

	{
		name: "blank_after_package_statement_no_comment",
		in: `package p

import "math"

var _ = fmt.Printf
`,
		out: `package p

import "fmt"

var _ = fmt.Printf
`,
	},

	{
		name: "cryptorand_preferred_easy_possible",
		in: `package p

var _ = rand.Read
`,
		out: `package p

import "crypto/rand"

var _ = rand.Read
`,
	},

	{
		name: "cryptorand_preferred_easy_impossible",
		in: `package p

var _ = rand.NewZipf
`,
		out: `package p

import "math/rand"

var _ = rand.NewZipf
`,
	},

	{
		name: "cryptorand_preferred_complex_possible",
		in: `package p

var _, _ = rand.Read, rand.Prime
`,
		out: `package p

import "crypto/rand"

var _, _ = rand.Read, rand.Prime
`,
	},

	{
		name: "cryptorand_preferred_complex_impossible",
		in: `package p

var _, _ = rand.Read, rand.NewZipf
`,
		out: `package p

import "math/rand"

var _, _ = rand.Read, rand.NewZipf
`,
	},
}

func TestSimpleCases(t *testing.T) {
	defer func(lp string) { LocalPrefix = lp }(LocalPrefix)
	LocalPrefix = "local,github.com/local"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &Options{
				TabWidth:   8,
				TabIndent:  true,
				Comments:   true,
				Fragment:   true,
				FormatOnly: tt.formatOnly,
			}
			testConfig{
				// Skeleton non-stdlib packages for use during testing.
				// Each includes one arbitrary symbol, e.g. the first declaration in the first file.
				// Try not to add more without a good reason.
				gopathFiles: map[string]string{
					"appengine/x.go":                          "package appengine\nfunc Main(){}\n",
					"appengine/datastore/x.go":                "package datastore\nvar ErrInvalidEntityType error\n",
					"rsc.io/p/x.go":                           "package p\nfunc P(){}\n",
					"code.google.com/p/snappy-go/snappy/x.go": "package snappy\nvar ErrCorrupt error\n",
					"x/x.go": tt.in,
				},
			}.processTest(t, "x/x.go", nil, options, tt.out)
		})
	}

}

func TestReadFromFilesystem(t *testing.T) {
	tests := []struct {
		name    string
		in, out string
	}{
		{
			name: "works",
			in: `package foo
func bar() {
fmt.Println("hi")
}
`,
			out: `package foo

import "fmt"

func bar() {
	fmt.Println("hi")
}
`,
		},
		{
			name: "missing_package",
			in: `
func bar() {
fmt.Println("hi")
}
`,
			out: `
import "fmt"

func bar() {
	fmt.Println("hi")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &Options{
				TabWidth:  8,
				TabIndent: true,
				Comments:  true,
				Fragment:  true,
			}
			testConfig{
				gopathFiles: map[string]string{
					"x.go": tt.in,
				},
			}.processTest(t, "x.go", nil, options, tt.out)
		})
	}

}

// Test support for packages in GOPATH that are actually symlinks.
// Also test that a symlink loop does not block the process.
func TestImportSymlinks(t *testing.T) {
	switch runtime.GOOS {
	case "windows", "plan9":
		t.Skipf("skipping test on %q as there are no symlinks", runtime.GOOS)
	}

	const input = `package p

var (
	_ = fmt.Print
	_ = mypkg.Foo
)
`
	const want = `package p

import (
	"fmt"
	"x/mypkg"
)

var (
	_ = fmt.Print
	_ = mypkg.Foo
)
`

	testConfig{
		gopathFiles: map[string]string{
			"../target/f.go":             "package mypkg\nvar Foo = 123\n",
			"x/mypkg":                    "LINK:../../target", // valid symlink
			"x/apkg":                     "LINK:..",           // symlink loop
			"myotherpackage/toformat.go": input,
		},
	}.processTest(t, "myotherpackage/toformat.go", nil, nil, want)
}

func TestImportSymlinksWithIgnore(t *testing.T) {
	switch runtime.GOOS {
	case "windows", "plan9":
		t.Skipf("skipping test on %q as there are no symlinks", runtime.GOOS)
	}

	const input = `package p

var (
	_ = fmt.Print
	_ = mypkg.Foo
)
`
	const want = `package p

import "fmt"

var (
	_ = fmt.Print
	_ = mypkg.Foo
)
`

	testConfig{
		gopathFiles: map[string]string{
			"../target/f.go":         "package mypkg\nvar Foo = 123\n",
			"x/mypkg":                "LINK:../../target", // valid symlink
			"x/apkg":                 "LINK:..",           // symlink loop
			"myotherpkg/toformat.go": input,
			".goimportsignore":       "x/mypkg\n",
		},
	}.processTest(t, "myotherpkg/toformat.go", nil, nil, want)
}

// Test for x/y/v2 convention for package y.
func TestModuleVersion(t *testing.T) {
	const input = `package p

import (
	"fmt"

	"github.com/foo/v2"
)

var (
	_ = fmt.Print
	_ = foo.Foo
)
`

	testConfig{
		gopathFiles: map[string]string{
			"mypkg.com/outpkg/toformat.go": input,
		},
	}.processTest(t, "mypkg.com/outpkg/toformat.go", nil, nil, input)
}

// Test for correctly identifying the name of a vendored package when it
// differs from its directory name. In this test, the import line
// "mypkg.com/mypkg.v1" would be removed if goimports wasn't able to detect
// that the package name is "mypkg".
func TestVendorPackage(t *testing.T) {
	const input = `package p

import (
	"fmt"

	"mypkg.com/mypkg.v1"
)

var (
	_ = fmt.Print
	_ = mypkg.Foo
)
`
	testConfig{
		gopathFiles: map[string]string{
			"mypkg.com/outpkg/vendor/mypkg.com/mypkg.v1/f.go": "package mypkg\nvar Foo = 123\n",
			"mypkg.com/outpkg/toformat.go":                    input,
		},
	}.processTest(t, "mypkg.com/outpkg/toformat.go", nil, nil, input)
}

func TestInternal(t *testing.T) {
	const input = `package bar

var _ = race.Acquire
`
	const importAdded = `package bar

import "foo/internal/race"

var _ = race.Acquire
`

	// Packages under the same directory should be able to use internal packages.
	testConfig{
		gopathFiles: map[string]string{
			"foo/internal/race/x.go": "package race\n func Acquire(){}\n",
			"foo/bar/x.go":           input,
		},
	}.processTest(t, "foo/bar/x.go", nil, nil, importAdded)

	// Packages outside the same directory should not.
	testConfig{
		gopathFiles: map[string]string{
			"foo/internal/race/x.go": "package race\n func Acquire(){}\n",
			"bar/x.go":               input,
		},
	}.processTest(t, "bar/x.go", nil, nil, input)
}

func TestProcessVendor(t *testing.T) {
	const input = `package p

var _ = hpack.HuffmanDecode
`
	const want = `package p

import "golang.org/x/net/http2/hpack"

var _ = hpack.HuffmanDecode
`
	testConfig{
		gopathFiles: map[string]string{
			"vendor/golang.org/x/net/http2/hpack/huffman.go": "package hpack\nfunc HuffmanDecode() { }\n",
			"bar/x.go": input,
		},
	}.processTest(t, "bar/x.go", nil, nil, want)
}

func TestFindStdlib(t *testing.T) {
	tests := []struct {
		pkg     string
		symbols []string
		want    string
	}{
		{"http", []string{"Get"}, "net/http"},
		{"http", []string{"Get", "Post"}, "net/http"},
		{"http", []string{"Get", "Foo"}, ""},
		{"bytes", []string{"Buffer"}, "bytes"},
		{"ioutil", []string{"Discard"}, "io/ioutil"},
	}
	for _, tt := range tests {
		input := "package p\n"
		for _, sym := range tt.symbols {
			input += fmt.Sprintf("var _ = %s.%s\n", tt.pkg, sym)
		}
		buf, err := Process("x.go", []byte(input), &Options{})
		if err != nil {
			t.Fatal(err)
		}
		if got := string(buf); !strings.Contains(got, tt.want) {
			t.Errorf("Process(%q) = %q, wanted it to contain %q", input, buf, tt.want)
		}
	}
}

type testConfig struct {
	// goroot and gopath optionally specifies the path on disk
	// to use for the GOROOT and GOPATH. If empty, a temp directory
	// is made if needed.
	goroot, gopath string

	// gorootFiles optionally specifies the complete contents of GOROOT to use,
	// If nil, the normal current $GOROOT is used.
	gorootFiles map[string]string // paths relative to $GOROOT/src to contents

	// gopathFiles is like gorootFiles, but for $GOPATH.
	// If nil, there is no GOPATH, though.
	gopathFiles map[string]string // paths relative to $GOPATH/src to contents
}

func mustTempDir(t *testing.T, prefix string) string {
	t.Helper()
	dir, err := ioutil.TempDir("", prefix)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func mapToDir(destDir string, files map[string]string) error {
	for path, contents := range files {
		file := filepath.Join(destDir, "src", path)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			return err
		}
		var err error
		if strings.HasPrefix(contents, "LINK:") {
			err = os.Symlink(strings.TrimPrefix(contents, "LINK:"), file)
		} else {
			err = ioutil.WriteFile(file, []byte(contents), 0644)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c testConfig) test(t *testing.T, fn func(*goimportTest)) {
	t.Helper()

	goroot := c.goroot
	gopath := c.gopath

	if c.gorootFiles != nil && goroot == "" {
		goroot = mustTempDir(t, "goroot-")
		defer os.RemoveAll(goroot)
	}
	if err := mapToDir(goroot, c.gorootFiles); err != nil {
		t.Fatal(err)
	}

	if c.gopathFiles != nil && gopath == "" {
		gopath = mustTempDir(t, "gopath-")
		defer os.RemoveAll(gopath)
	}
	if err := mapToDir(gopath, c.gopathFiles); err != nil {
		t.Fatal(err)
	}

	scanOnce = sync.Once{}

	oldGOPATH := build.Default.GOPATH
	oldGOROOT := build.Default.GOROOT
	oldCompiler := build.Default.Compiler
	build.Default.GOPATH = ""
	build.Default.Compiler = "gc"

	defer func() {
		build.Default.GOPATH = oldGOPATH
		build.Default.GOROOT = oldGOROOT
		build.Default.Compiler = oldCompiler
	}()

	if goroot != "" {
		build.Default.GOROOT = goroot
	}
	build.Default.GOPATH = gopath

	it := &goimportTest{
		T:      t,
		goroot: build.Default.GOROOT,
		gopath: gopath,
		ctx:    &build.Default,
	}
	fn(it)
}

func (c testConfig) processTest(t *testing.T, file string, contents []byte, opts *Options, want string) {
	t.Helper()
	c.test(t, func(t *goimportTest) {
		t.Helper()
		t.process(file, contents, opts, want)
	})
}

type goimportTest struct {
	*testing.T
	ctx    *build.Context
	goroot string
	gopath string
}

func (t *goimportTest) process(file string, contents []byte, opts *Options, want string) {
	t.Helper()
	buf, err := Process(filepath.Join(t.gopath, "src", file), contents, opts)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != want {
		t.Errorf("Got:\n%s\nWant:\n%s", buf, want)
	}
}

// Tests that added imports are renamed when the import path's base doesn't
// match its package name. For example, we want to generate:
//
//     import cloudbilling "google.golang.org/api/cloudbilling/v1"
func TestRenameWhenPackageNameMismatch(t *testing.T) {
	const input = `package main
 const Y = bar.X`

	const want = `package main

import bar "foo/bar/v1"

const Y = bar.X
`
	testConfig{
		gopathFiles: map[string]string{
			"foo/bar/v1/x.go": "package bar \n const X = 1",
			"test/t.go":       input,
		},
	}.processTest(t, "test/t.go", nil, nil, want)
}

// Tests that the LocalPrefix option causes imports
// to be added into a later group (num=3).
func TestLocalPrefix(t *testing.T) {
	tests := []struct {
		config      testConfig
		localPrefix string
		src         string
		want        string
	}{
		{
			config: testConfig{
				gopathFiles: map[string]string{
					"foo/bar/bar.go": "package bar \n const X = 1",
				},
			},
			localPrefix: "foo/",
			src:         "package main \n const Y = bar.X \n const _ = runtime.GOOS",
			want: `package main

import (
	"runtime"

	"foo/bar"
)

const Y = bar.X
const _ = runtime.GOOS
`,
		},
		{
			config: testConfig{
				gopathFiles: map[string]string{
					"foo/foo.go":     "package foo \n const X = 1",
					"foo/bar/bar.go": "package bar \n const X = 1",
				},
			},
			localPrefix: "foo/",
			src:         "package main \n const Y = bar.X \n const Z = foo.X \n const _ = runtime.GOOS",
			want: `package main

import (
	"runtime"

	"foo"
	"foo/bar"
)

const Y = bar.X
const Z = foo.X
const _ = runtime.GOOS
`,
		},
		{
			config: testConfig{
				gopathFiles: map[string]string{
					"example.org/pkg/pkg.go":          "package pkg \n const A = 1",
					"foo/bar/bar.go":                  "package bar \n const B = 1",
					"code.org/r/p/expproj/expproj.go": "package expproj \n const C = 1",
				},
			},
			localPrefix: "example.org/pkg,foo/,code.org",
			src:         "package main \n const X = pkg.A \n const Y = bar.B \n const Z = expproj.C \n const _ = runtime.GOOS",
			want: `package main

import (
	"runtime"

	"code.org/r/p/expproj"
	"example.org/pkg"
	"foo/bar"
)

const X = pkg.A
const Y = bar.B
const Z = expproj.C
const _ = runtime.GOOS
`,
		},
	}

	for _, tt := range tests {
		tt.config.test(t, func(t *goimportTest) {
			defer func(s string) { LocalPrefix = s }(LocalPrefix)
			LocalPrefix = tt.localPrefix
			t.process("test/t.go", []byte(tt.src), nil, tt.want)
		})
	}
}

// Tests that "package documentation" files are ignored.
func TestIgnoreDocumentationPackage(t *testing.T) {
	const input = `package x

const Y = foo.X
`
	const want = `package x

import "foo"

const Y = foo.X
`

	testConfig{
		gopathFiles: map[string]string{
			"foo/foo.go": "package foo\nconst X = 1\n",
			"foo/doc.go": "package documentation \n // just to confuse things\n",
			"x/x.go":     input,
		},
	}.processTest(t, "x/x.go", nil, nil, want)
}

// Tests importPathToNameGoPathParse and in particular that it stops
// after finding the first non-documentation package name, not
// reporting an error on inconsistent package names (since it should
// never make it that far).
func TestImportPathToNameGoPathParse(t *testing.T) {
	testConfig{
		gopathFiles: map[string]string{
			"example.net/pkg/doc.go": "package documentation\n", // ignored
			"example.net/pkg/gen.go": "package main\n",          // also ignored
			"example.net/pkg/pkg.go": "package the_pkg_name_to_find\n  and this syntax error is ignored because of parser.PackageClauseOnly",
			"example.net/pkg/z.go":   "package inconsistent\n", // inconsistent but ignored
		},
	}.test(t, func(t *goimportTest) {
		got, err := importPathToNameGoPathParse("example.net/pkg", filepath.Join(t.gopath, "src", "other.net"))
		if err != nil {
			t.Fatal(err)
		}
		const want = "the_pkg_name_to_find"
		if got != want {
			t.Errorf("importPathToNameGoPathParse(..) = %q; want %q", got, want)
		}
	})
}

func TestIgnoreConfiguration(t *testing.T) {
	const input = `package x

const _ = pkg.X
`
	const want = `package x

import "otherwise-longer-so-worse.example.net/foo/pkg"

const _ = pkg.X
`

	testConfig{
		gopathFiles: map[string]string{
			".goimportsignore":       "# comment line\n\n example.net", // tests comment, blank line, whitespace trimming
			"example.net/pkg/pkg.go": "package pkg\nconst X = 1",
			"otherwise-longer-so-worse.example.net/foo/pkg/pkg.go": "package pkg\nconst X = 1",
			"x/x.go": input,
		},
	}.processTest(t, "x/x.go", nil, nil, want)
}

// Skip "node_modules" directory.
func TestSkipNodeModules(t *testing.T) {
	const input = `package x

const _ = pkg.X
`
	const want = `package x

import "otherwise-longer.net/not_modules/pkg"

const _ = pkg.X
`

	testConfig{
		gopathFiles: map[string]string{
			"example.net/node_modules/pkg/a.go":         "package pkg\nconst X = 1",
			"otherwise-longer.net/not_modules/pkg/a.go": "package pkg\nconst X = 1",
			"x/x.go": input,
		},
	}.processTest(t, "x/x.go", nil, nil, want)
}

// golang.org/issue/16458 -- if GOROOT is a prefix of GOPATH, GOPATH is ignored.
func TestGoRootPrefixOfGoPath(t *testing.T) {
	const input = `package x

const _ = foo.X
`
	const want = `package x

import "example.com/foo"

const _ = foo.X
`

	dir := mustTempDir(t, "importstest")
	defer os.RemoveAll(dir)
	testConfig{
		goroot: filepath.Join(dir, "go"),
		gopath: filepath.Join(dir, "gopath"),
		gopathFiles: map[string]string{
			"example.com/foo/pkg.go": "package foo\nconst X = 1",
			"x/x.go":                 input,
		},
	}.processTest(t, "x/x.go", nil, nil, want)
}

// Tests that package global variables with the same name and function name as
// a function in a separate package do not result in an import which masks
// the global variable
func TestGlobalImports(t *testing.T) {
	const usesGlobal = `package pkg

func doSomething() {
	t := time.Now()
}
`

	const declaresGlobal = `package pkg

type Time struct{}

func (t Time) Now() Time {
	return Time{}
}

var time Time
`

	testConfig{
		gopathFiles: map[string]string{
			"pkg/uses.go":   usesGlobal,
			"pkg/global.go": declaresGlobal,
		},
	}.processTest(t, "pkg/uses.go", nil, nil, usesGlobal)
}

// Tests that sibling files - other files in the same package - can provide an
// import that may not be the default one otherwise.
func TestSiblingImports(t *testing.T) {

	// provide is the sibling file that provides the desired import.
	const provide = `package siblingimporttest

import "local/log"
import "my/bytes"

func LogSomething() {
	log.Print("Something")
	bytes.SomeFunc()
}
`

	// need is the file being tested that needs the import.
	const need = `package siblingimporttest

var _ = bytes.Buffer{}

func LogSomethingElse() {
	log.Print("Something else")
}
`

	// want is the expected result file
	const want = `package siblingimporttest

import (
	"bytes"
	"local/log"
)

var _ = bytes.Buffer{}

func LogSomethingElse() {
	log.Print("Something else")
}
`

	testConfig{
		gopathFiles: map[string]string{
			"p/needs_import.go":    need,
			"p/provides_import.go": provide,
		},
	}.processTest(t, "p/needs_import.go", nil, nil, want)
}

func TestPkgIsCandidate(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		pkgIdent string
		pkg      *pkg
		want     bool
	}{
		{
			name:     "normal_match",
			filename: "/gopath/src/my/pkg/pkg.go",
			pkgIdent: "client",
			pkg: &pkg{
				dir:             "/gopath/src/client",
				importPath:      "client",
				importPathShort: "client",
			},
			want: true,
		},
		{
			name:     "no_match",
			filename: "/gopath/src/my/pkg/pkg.go",
			pkgIdent: "zzz",
			pkg: &pkg{
				dir:             "/gopath/src/client",
				importPath:      "client",
				importPathShort: "client",
			},
			want: false,
		},
		{
			name:     "match_too_early",
			filename: "/gopath/src/my/pkg/pkg.go",
			pkgIdent: "client",
			pkg: &pkg{
				dir:             "/gopath/src/client/foo/foo/foo",
				importPath:      "client/foo/foo",
				importPathShort: "client/foo/foo",
			},
			want: false,
		},
		{
			name:     "substring_match",
			filename: "/gopath/src/my/pkg/pkg.go",
			pkgIdent: "client",
			pkg: &pkg{
				dir:             "/gopath/src/foo/go-client",
				importPath:      "foo/go-client",
				importPathShort: "foo/go-client",
			},
			want: true,
		},
		{
			name:     "hidden_internal",
			filename: "/gopath/src/my/pkg/pkg.go",
			pkgIdent: "client",
			pkg: &pkg{
				dir:             "/gopath/src/foo/internal/client",
				importPath:      "foo/internal/client",
				importPathShort: "foo/internal/client",
			},
			want: false,
		},
		{
			name:     "visible_internal",
			filename: "/gopath/src/foo/bar.go",
			pkgIdent: "client",
			pkg: &pkg{
				dir:             "/gopath/src/foo/internal/client",
				importPath:      "foo/internal/client",
				importPathShort: "foo/internal/client",
			},
			want: true,
		},
		{
			name:     "invisible_vendor",
			filename: "/gopath/src/foo/bar.go",
			pkgIdent: "client",
			pkg: &pkg{
				dir:             "/gopath/src/other/vendor/client",
				importPath:      "other/vendor/client",
				importPathShort: "client",
			},
			want: false,
		},
		{
			name:     "visible_vendor",
			filename: "/gopath/src/foo/bar.go",
			pkgIdent: "client",
			pkg: &pkg{
				dir:             "/gopath/src/foo/vendor/client",
				importPath:      "other/foo/client",
				importPathShort: "client",
			},
			want: true,
		},
		{
			name:     "match_with_hyphens",
			filename: "/gopath/src/foo/bar.go",
			pkgIdent: "socketio",
			pkg: &pkg{
				dir:             "/gopath/src/foo/socket-io",
				importPath:      "foo/socket-io",
				importPathShort: "foo/socket-io",
			},
			want: true,
		},
		{
			name:     "match_with_mixed_case",
			filename: "/gopath/src/foo/bar.go",
			pkgIdent: "fooprod",
			pkg: &pkg{
				dir:             "/gopath/src/foo/FooPROD",
				importPath:      "foo/FooPROD",
				importPathShort: "foo/FooPROD",
			},
			want: true,
		},
		{
			name:     "matches_with_hyphen_and_caps",
			filename: "/gopath/src/foo/bar.go",
			pkgIdent: "fooprod",
			pkg: &pkg{
				dir:             "/gopath/src/foo/Foo-PROD",
				importPath:      "foo/Foo-PROD",
				importPathShort: "foo/Foo-PROD",
			},
			want: true,
		},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pkgIsCandidate(tt.filename, tt.pkgIdent, tt.pkg)
			if got != tt.want {
				t.Errorf("test %d. pkgIsCandidate(%q, %q, %+v) = %v; want %v",
					i, tt.filename, tt.pkgIdent, *tt.pkg, got, tt.want)
			}
		})
	}
}

// Issue 20941: this used to panic on Windows.
func TestProcessStdin(t *testing.T) {
	got, err := Process("<standard input>", []byte("package main\nfunc main() {\n\tfmt.Println(123)\n}\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"fmt"`) {
		t.Errorf("expected fmt import; got: %s", got)
	}
}

// Tests LocalPackagePromotion when there is a local package that matches, it
// should be the closest match.
// https://golang.org/issues/17557
func TestLocalPackagePromotion(t *testing.T) {
	const input = `package main
var c = &config.SystemConfig{}
`
	const want = `package main

import "mycompany.net/tool/config"

var c = &config.SystemConfig{}
`

	testConfig{
		gopathFiles: map[string]string{
			"config.net/config/config.go":         "package config\n type SystemConfig struct {}", // Will match but should not be first choice
			"mycompany.net/config/config.go":      "package config\n type SystemConfig struct {}", // Will match but should not be first choice
			"mycompany.net/tool/config/config.go": "package config\n type SystemConfig struct {}", // Local package should be promoted over shorter package
			"mycompany.net/tool/main.go":          input,
		},
	}.processTest(t, "mycompany.net/tool/main.go", nil, nil, want)
}

// Tests FindImportInLocalGoFiles looks at the import lines for other Go files in the
// local directory, since the user is likely to import the same packages in the current
// Go file.  If an import is found that satisfies the need, it should be used over the
// standard library.
// https://golang.org/issues/17557
func TestFindImportInLocalGoFiles(t *testing.T) {
	const input = `package main
 var _ = &bytes.Buffer{}`

	const want = `package main

import "bytes.net/bytes"

var _ = &bytes.Buffer{}
`
	testConfig{
		gopathFiles: map[string]string{
			"bytes.net/bytes/bytes.go":   "package bytes\n type Buffer struct {}",                               // Should be selected over standard library
			"mycompany.net/tool/io.go":   "package main\n import \"bytes.net/bytes\"\n var _ = &bytes.Buffer{}", // Contains package import that will cause stdlib to be ignored
			"mycompany.net/tool/err.go":  "package main\n import \"bogus.net/bytes\"\n var _ = &bytes.Buffer{}", // Contains import which is not resolved, so it is ignored
			"mycompany.net/tool/main.go": input,
		},
	}.processTest(t, "mycompany.net/tool/main.go", nil, nil, want)
}

func TestImportNoGoFiles(t *testing.T) {
	const input = `package main
 var _ = &bytes.Buffer{}`

	const want = `package main

import "bytes"

var _ = &bytes.Buffer{}
`

	testConfig{}.processTest(t, "mycompany.net/tool/main.go", []byte(input), nil, want)
}

// Ensures a token as large as 500000 bytes can be handled
// https://golang.org/issues/18201
func TestProcessLargeToken(t *testing.T) {
	largeString := strings.Repeat("x", 500000)

	input := `package testimports

import (
	"fmt"
	"mydomain.mystuff/mypkg"
)

const s = fmt.Sprintf("%s", "` + largeString + `")
const x = mypkg.Sprintf("%s", "my package")

// end
`

	want := `package testimports

import (
	"fmt"

	"mydomain.mystuff/mypkg"
)

const s = fmt.Sprintf("%s", "` + largeString + `")
const x = mypkg.Sprintf("%s", "my package")

// end
`

	testConfig{
		gopathFiles: map[string]string{"foo.go": input},
	}.processTest(t, "foo.go", nil, nil, want)
}
