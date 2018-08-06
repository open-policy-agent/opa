// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package diff

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
)

func writeTempFile(dir, prefix string, data []byte) (string, error) {
	file, err := ioutil.TempFile(dir, prefix)
	if err != nil {
		return "", err
	}
	_, err = file.Write(data)
	if err1 := file.Close(); err == nil {
		err = err1
	}
	if err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

// Diff returns a human-readable description of the differences between s1 and s2.
// If the "diff" command is available, it returns the output of unified diff on s1 and s2.
// If the result is non-empty, the strings differ or the diff command failed.
func Diff(s1, s2 string) string {
	if s1 == s2 {
		return ""
	}
	if _, err := exec.LookPath("diff"); err != nil {
		return fmt.Sprintf("diff command unavailable\nold: %q\nnew: %q", s1, s2)
	}
	f1, err := writeTempFile("", "benchfmt_test", []byte(s1))
	if err != nil {
		return err.Error()
	}
	defer os.Remove(f1)

	f2, err := writeTempFile("", "benchfmt_test", []byte(s2))
	if err != nil {
		return err.Error()
	}
	defer os.Remove(f2)

	cmd := "diff"
	if runtime.GOOS == "plan9" {
		cmd = "/bin/ape/diff"
	}

	data, err := exec.Command(cmd, "-u", f1, f2).CombinedOutput()
	if len(data) > 0 {
		// diff exits with a non-zero status when the files don't match.
		// Ignore that failure as long as we get output.
		err = nil
	}
	if err != nil {
		data = append(data, []byte(err.Error())...)
	}
	return string(data)
}
