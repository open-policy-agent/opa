package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/OneOfOne/xxhash"
)

var (
	mux      sync.Mutex
	errored  bool
	use32    = flag.Bool("32", false, "use 32bit hash instead of 64bit")
	checkArg = flag.Bool("c", false, "read and check the sums of input files")
	seedArg  = flag.Uint64("s", 0, "use `seed` to seed the hasher")
)

func init() {
	flag.Parse()
	flag.Usage = func() {
		errorf("Usage of %s: [-32] [-c] [-s seed] files...\t%s *.go > sums.xx\t%s -c sums.xx", os.Args[0], os.Args[0], os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func main() {
	args := flag.Args()
	st, _ := os.Stdin.Stat()
	if st.Mode()&os.ModeCharDevice == 0 {
		args = append(args, "-")
	}
	if len(args) == 0 {
		flag.Usage()
	}
	sema := newSema(runtime.NumCPU())
	var wg sync.WaitGroup
	wg.Add(len(args))
	if !*checkArg {
		printf("# seed %d", *seedArg)
		if *use32 {
			printf("# 32bit")
		} else {
			printf("# 64bit")
		}
	}
	for _, fn := range args {
		if *checkArg {
			check(newSema(runtime.NumCPU()), fn)
		} else {
			sema.Run(func() { printHash(fn) })
		}
	}
	sema.WaitAndClose()
	if errored {
		os.Exit(1)
	}
}

func check(sema *sema, fn string) {
	defer sema.WaitAndClose()
	var err error
	var f *os.File
	if fn == "-" {
		f = os.Stdin
	} else {
		if f, err = os.Open(fn); err != nil {
			errorf("error opening %s: %v", fn, err)
			return
		}
		defer f.Close()
	}
	buf := bufio.NewScanner(f)
	for buf.Scan() {
		ln := strings.TrimSpace(buf.Text())
		if len(ln) == 0 {
			continue
		}
		if strings.HasPrefix(ln, "# seed ") {
			*seedArg, _ = strconv.ParseUint(strings.TrimSpace(ln[7:]), 10, 64)
			continue
		}
		if strings.HasPrefix(ln, "# 32bit") {
			*use32 = true
			continue
		}
		if strings.HasPrefix(ln, "# 64bit") {
			*use32 = false
			continue
		}
		if ln[0] == '#' {
			continue
		}
		parts := strings.SplitN(ln, "\t", 2)
		if len(parts) != 2 || len(parts[0]) < 1 {
			continue
		}
		oh, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil {
			errorf("error: %v", err)
			continue
		}
		sema.Run(func() {
			fn := strings.TrimSpace(parts[1])
			nh, err := hashFile(fn)
			if err != nil {
				errorf("error hashing %s: %v", fn, err)
			}
			if oh != nh {
				errorf("hash mismatch %q 0x%X 0x%X", fn, oh, nh)
			}
		})
	}
}

func printHash(fn string) {
	h, err := hashFile(fn)
	if err != nil {
		errorf("error hashing %s: %v", fn, err)
		return
	}
	if h == 0 {
		return
	}
	if *use32 {
		printf("%-10d\t%s", h, fn)
	} else {
		printf("%-20d\t%s", h, fn)
	}
}

func hashFile(fn string) (h uint64, err error) {
	var f *os.File
	if fn == "-" {
		f = os.Stdin
	} else {
		if f, err = os.Open(fn); err != nil {
			return
		}
		defer f.Close()
		if st, _ := f.Stat(); st.IsDir() || st.Size() == 0 {
			return
		}
	}
	if *use32 {
		xx := xxhash.NewS32(uint32(*seedArg))
		if _, err = io.Copy(xx, f); err != nil {
			return
		}
		return uint64(xx.Sum32()), nil
	}
	xx := xxhash.NewS64(*seedArg)
	if _, err = io.Copy(xx, f); err != nil {
		return
	}
	return xx.Sum64(), nil
}

func printf(f string, args ...interface{}) {
	mux.Lock()
	fmt.Fprintf(os.Stdout, f+"\n", args...)
	mux.Unlock()
}

func errorf(f string, args ...interface{}) {
	mux.Lock()
	fmt.Fprintf(os.Stderr, f+"\n", args...)
	errored = true
	mux.Unlock()
}
