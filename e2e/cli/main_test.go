// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cli

import (
	"cmp"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestScript(t *testing.T) {
	opaBin := cmp.Or(os.Getenv("OPA"), "opa")

	testscript.Run(t, testscript.Params{
		Dir: ".",
		Setup: func(e *testscript.Env) error {
			e.Vars = append(e.Vars, "OPA="+opaBin)
			return nil
		},
		Cmds: map[string]func(*testscript.TestScript, bool, []string){
			"retry":       retryCmd,
			"http_server": httpServerCmd,
			"jq":          jqCmd,
		},
		UpdateScripts: os.Getenv("E2E_UPDATE") != "",
	})
}

func retryCmd(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) == 0 {
		ts.Fatalf("usage: retry command [args...]")
	}

	const maxRetries = 30
	const delay = 1 * time.Second

	var lastErr error
	for i := range maxRetries {
		if i > 0 {
			time.Sleep(delay)
		}

		err := ts.Exec(args[0], args[1:]...)
		if err == nil {
			if neg {
				ts.Fatalf("unexpected command success")
			}
			return
		}
		lastErr = err
	}

	if neg {
		return
	}

	ts.Fatalf("command failed after %d attempts: %v", maxRetries, lastErr)
}

func httpServerCmd(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("http_server does not support negation")
	}
	if len(args) != 1 {
		ts.Fatalf("usage: http_server <dir>")
	}

	dir := ts.MkAbs(args[0])

	mime.AddExtensionType(".gz", "application/octet-stream")

	lis, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		ts.Fatalf("failed to listen: %v", err)
	}

	srv := &http.Server{Handler: http.FileServer(http.Dir(dir))}
	go srv.Serve(lis)

	ts.Defer(func() {
		srv.Close()
	})

	ts.Setenv("HTTP_ADDR", lis.Addr().String())
}

func jqCmd(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 2 {
		ts.Fatalf("usage: jq [-s] stdout|stderr|<file> <expression>")
	}

	var jqArgs []string
	jqArgs = append(jqArgs, "-e")

	i := 0
	for i < len(args)-2 {
		jqArgs = append(jqArgs, args[i])
		i++
	}

	source := args[i]
	expr := args[i+1]
	input := ts.ReadFile(source)

	jqArgs = append(jqArgs, expr)
	cmd := exec.Command("jq", jqArgs...)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()

	if neg {
		if err == nil {
			ts.Fatalf("jq expression matched unexpectedly: %s", out)
		}
	} else {
		if err != nil {
			ts.Fatalf("jq expression did not match: %v\n%s was: %s", err, source, input)
		}
	}
}
