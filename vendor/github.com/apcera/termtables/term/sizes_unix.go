// Copyright 2013 Apcera Inc. All rights reserved.

// +build cgo,!windows

package term

/*
#include <termios.h>
#include <sys/ioctl.h>

// provides struct winsize *, with:
//    ws_row, ws_col, ws_xpixel, ws_ypixel
// all short
*/
import "C"

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// ErrGetWinsizeFailed indicates that the system call to extract the size of
// a Unix tty from the kernel failed.
var ErrGetWinsizeFailed = errors.New("term: syscall.TIOCGWINSZ failed")

// GetTerminalWindowSize returns the terminal size maintained by the kernel
// for a Unix TTY, passed in as an *os.File.  This information can be seen
// with the stty(1) command, and changes in size (eg, terminal emulator
// resized) should trigger a SIGWINCH signal delivery to the foreground process
// group at the time of the change, so a long-running process might reasonably
// watch for SIGWINCH and arrange to re-fetch the size when that happens.
func GetTerminalWindowSize(file *os.File) (*Size, error) {
	fd := uintptr(file.Fd())
	winsize := C.struct_winsize{}
	winp := uintptr(unsafe.Pointer(&winsize))
	_, _, ep := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ, winp)
	if ep != 0 {
		return nil, ErrGetWinsizeFailed
	}
	return &Size{
		Lines:   int(winsize.ws_row),
		Columns: int(winsize.ws_col),
	}, nil
}
