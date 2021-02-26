package wasm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/internal/wasm/encoding"
)

const warning = `---------------------------------------------------------------
WARNING: Using EXPERIMENTAL, unsupported wasm-opt optimization.
         It is not supported, and may go away in the future.
---------------------------------------------------------------`

// optimizeBinaryen passes the encoded module into wasm-opt, and replaces
// the compiler's module with the decoding of the process' output.
func (c *Compiler) optimizeBinaryen() error {
	if os.Getenv("EXPERIMENTAL_WASM_OPT") == "" && os.Getenv("EXPERIMENTAL_WASM_OPT_ARGS") == "" {
		c.debug.Printf("not opted in, skipping wasm-opt optimization")
		return nil
	}
	if !woptFound() {
		c.debug.Printf("wasm-opt binary not found, skipping optimization")
		return nil
	}
	fmt.Fprintln(os.Stderr, warning)
	args := []string{ // WARNING: flags with typos are ignored!
		"-O2",
		"--debuginfo", // don't strip name section
	}
	// allow overriding the options
	if env := os.Getenv("EXPERIMENTAL_WASM_OPT_ARGS"); env != "" {
		args = strings.Split(env, " ")
	}

	args = append(args, "-o", "-") // always output to stdout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wopt := exec.CommandContext(ctx, "wasm-opt", args...)
	stdin, err := wopt.StdinPipe()
	if err != nil {
		return fmt.Errorf("get stdin: %w", err)
	}
	defer stdin.Close()

	var stdout, stderr bytes.Buffer
	wopt.Stdout = &stdout

	if err := wopt.Start(); err != nil {
		return fmt.Errorf("start wasm-opt: %w", err)
	}
	if err := encoding.WriteModule(stdin, c.module); err != nil {
		return fmt.Errorf("encode module: %w", err)
	}
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("write to wasm-opt: %w", err)
	}
	if err := wopt.Wait(); err != nil {
		return fmt.Errorf("wait for wasm-opt: %w", err)
	}

	if d := stderr.String(); d != "" {
		c.debug.Printf("wasm-opt debug output: %s", d)
	}
	mod, err := encoding.ReadModule(&stdout)
	if err != nil {
		return fmt.Errorf("decode module: %w", err)
	}
	c.module = mod
	return nil
}

func woptFound() bool {
	_, err := exec.LookPath("wasm-opt")
	return err == nil
}
