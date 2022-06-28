// Package sys includes constants and types used by both public and internal APIs.
package sys

import (
	"fmt"
)

// ExitError is returned to a caller of api.Function still running when api.Module CloseWithExitCode was invoked.
// ExitCode zero value means success, while any other value is an error.
//
// Here's an example of how to get the exit code:
//	main := module.ExportedFunction("main")
//	if err := main(nil); err != nil {
//		if exitErr, ok := err.(*sys.ExitError); ok {
//			// If your main function expects to exit, this could be ok if Code == 0
//		}
//	--snip--
//
// Note: While possible the reason of this was "proc_exit" from "wasi_snapshot_preview1", it could be from other host
// functions, for example an AssemblyScript's abort handler, or any arbitrary caller of CloseWithExitCode.
//
// See https://github.com/WebAssembly/WASI/blob/main/phases/snapshot/docs.md#proc_exit and
// https://www.assemblyscript.org/concepts.html#special-imports
type ExitError struct {
	moduleName string
	exitCode   uint32
}

func NewExitError(moduleName string, exitCode uint32) *ExitError {
	return &ExitError{moduleName: moduleName, exitCode: exitCode}
}

// ModuleName is the api.Module that was closed.
func (e *ExitError) ModuleName() string {
	return e.moduleName
}

// ExitCode returns zero on success, and an arbitrary value otherwise.
func (e *ExitError) ExitCode() uint32 {
	return e.exitCode
}

// Error implements the error interface.
func (e *ExitError) Error() string {
	return fmt.Sprintf("module %q closed with exit_code(%d)", e.moduleName, e.exitCode)
}

// Is allows use via errors.Is
func (e *ExitError) Is(err error) bool {
	if target, ok := err.(*ExitError); ok {
		return e.moduleName == target.moduleName && e.exitCode == target.exitCode
	}
	return false
}
