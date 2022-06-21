package sys

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/tetratelabs/wazero/internal/platform"
	"github.com/tetratelabs/wazero/sys"
)

// Context holds module-scoped system resources currently only supported by
// built-in host functions.
type Context struct {
	args, environ         []string
	argsSize, environSize uint32
	stdin                 io.Reader
	stdout, stderr        io.Writer

	// Note: Using function pointers here keeps them stable for tests.

	walltime           *sys.Walltime
	walltimeResolution sys.ClockResolution
	nanotime           *sys.Nanotime
	nanotimeResolution sys.ClockResolution
	nanosleep          *sys.Nanosleep
	randSource         io.Reader

	fs *FSContext
}

// Args is like os.Args and defaults to nil.
//
// Note: The count will never be more than math.MaxUint32.
// See wazero.ModuleConfig WithArgs
func (c *Context) Args() []string {
	return c.args
}

// ArgsSize is the size to encode Args as Null-terminated strings.
//
// Note: To get the size without null-terminators, subtract the length of Args from this value.
// See wazero.ModuleConfig WithArgs
// See https://en.wikipedia.org/wiki/Null-terminated_string
func (c *Context) ArgsSize() uint32 {
	return c.argsSize
}

// Environ are "key=value" entries like os.Environ and default to nil.
//
// Note: The count will never be more than math.MaxUint32.
// See wazero.ModuleConfig WithEnv
func (c *Context) Environ() []string {
	return c.environ
}

// EnvironSize is the size to encode Environ as Null-terminated strings.
//
// Note: To get the size without null-terminators, subtract the length of Environ from this value.
// See wazero.ModuleConfig WithEnv
// See https://en.wikipedia.org/wiki/Null-terminated_string
func (c *Context) EnvironSize() uint32 {
	return c.environSize
}

// Stdin is like exec.Cmd Stdin and defaults to a reader of os.DevNull.
// See wazero.ModuleConfig WithStdin
func (c *Context) Stdin() io.Reader {
	return c.stdin
}

// Stdout is like exec.Cmd Stdout and defaults to io.Discard.
// See wazero.ModuleConfig WithStdout
func (c *Context) Stdout() io.Writer {
	return c.stdout
}

// Stderr is like exec.Cmd Stderr and defaults to io.Discard.
// See wazero.ModuleConfig WithStderr
func (c *Context) Stderr() io.Writer {
	return c.stderr
}

// Walltime implements sys.Walltime.
func (c *Context) Walltime(ctx context.Context) (sec int64, nsec int32) {
	return (*(c.walltime))(ctx)
}

// WalltimeResolution returns resolution of Walltime.
func (c *Context) WalltimeResolution() sys.ClockResolution {
	return c.walltimeResolution
}

// Nanotime implements sys.Nanotime.
func (c *Context) Nanotime(ctx context.Context) int64 {
	return (*(c.nanotime))(ctx)
}

// NanotimeResolution returns resolution of Nanotime.
func (c *Context) NanotimeResolution() sys.ClockResolution {
	return c.nanotimeResolution
}

// Nanosleep implements sys.Nanosleep.
func (c *Context) Nanosleep(ctx context.Context, ns int64) {
	(*(c.nanosleep))(ctx, ns)
}

// FS returns the file system context.
func (c *Context) FS(ctx context.Context) *FSContext {
	// Override Context when it is passed via context
	if fsValue := ctx.Value(FSKey{}); fsValue != nil {
		fsCtx, ok := fsValue.(*FSContext)
		if !ok {
			panic(fmt.Errorf("unsupported fs key: %v", fsValue))
		}
		return fsCtx
	}
	return c.fs
}

// RandSource is a source of random bytes and defaults to crypto/rand.Reader.
// see wazero.ModuleConfig WithRandSource
func (c *Context) RandSource() io.Reader {
	return c.randSource
}

// eofReader is safer than reading from os.DevNull as it can never overrun operating system file descriptors.
type eofReader struct{}

// Read implements io.Reader
// Note: This doesn't use a pointer reference as it has no state and an empty struct doesn't allocate.
func (eofReader) Read([]byte) (int, error) {
	return 0, io.EOF
}

// DefaultContext returns Context with no values set.
//
// Note: This isn't a constant because Context.openedFiles is currently mutable even when empty.
// TODO: Make it an error to open or close files when no FS was assigned.
func DefaultContext() *Context {
	if sysCtx, err := NewContext(0, nil, nil, nil, nil, nil, nil, nil, 0, nil, 0, nil, nil); err != nil {
		panic(fmt.Errorf("BUG: DefaultContext should never error: %w", err))
	} else {
		return sysCtx
	}
}

var _ = DefaultContext() // Force panic on bug.
var ns sys.Nanosleep = platform.FakeNanosleep

// NewContext is a factory function which helps avoid needing to know defaults or exporting all fields.
// Note: max is exposed for testing. max is only used for env/args validation.
func NewContext(
	max uint32,
	args, environ []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
	randSource io.Reader,
	walltime *sys.Walltime, walltimeResolution sys.ClockResolution,
	nanotime *sys.Nanotime, nanotimeResolution sys.ClockResolution,
	nanosleep *sys.Nanosleep,
	openedFiles map[uint32]*FileEntry,
) (sysCtx *Context, err error) {
	sysCtx = &Context{args: args, environ: environ}

	if sysCtx.argsSize, err = nullTerminatedByteCount(max, args); err != nil {
		return nil, fmt.Errorf("args invalid: %w", err)
	}

	if sysCtx.environSize, err = nullTerminatedByteCount(max, environ); err != nil {
		return nil, fmt.Errorf("environ invalid: %w", err)
	}

	if stdin == nil {
		sysCtx.stdin = eofReader{}
	} else {
		sysCtx.stdin = stdin
	}

	if stdout == nil {
		sysCtx.stdout = io.Discard
	} else {
		sysCtx.stdout = stdout
	}

	if stderr == nil {
		sysCtx.stderr = io.Discard
	} else {
		sysCtx.stderr = stderr
	}

	if randSource == nil {
		sysCtx.randSource = rand.Reader
	} else {
		sysCtx.randSource = randSource
	}

	if walltime != nil {
		if clockResolutionInvalid(walltimeResolution) {
			return nil, fmt.Errorf("invalid Walltime resolution: %d", walltimeResolution)
		}
		sysCtx.walltime = walltime
		sysCtx.walltimeResolution = walltimeResolution
	} else {
		sysCtx.walltime = platform.NewFakeWalltime()
		sysCtx.walltimeResolution = sys.ClockResolution(time.Microsecond.Nanoseconds())
	}

	if nanotime != nil {
		if clockResolutionInvalid(nanotimeResolution) {
			return nil, fmt.Errorf("invalid Nanotime resolution: %d", nanotimeResolution)
		}
		sysCtx.nanotime = nanotime
		sysCtx.nanotimeResolution = nanotimeResolution
	} else {
		sysCtx.nanotime = platform.NewFakeNanotime()
		sysCtx.nanotimeResolution = sys.ClockResolution(time.Nanosecond)
	}

	if nanosleep != nil {
		sysCtx.nanosleep = nanosleep
	} else {
		sysCtx.nanosleep = &ns
	}

	sysCtx.fs = NewFSContext(openedFiles)

	return
}

// clockResolutionInvalid returns true if the value stored isn't reasonable.
func clockResolutionInvalid(resolution sys.ClockResolution) bool {
	return resolution < 1 || resolution > sys.ClockResolution(time.Hour.Nanoseconds())
}

// nullTerminatedByteCount ensures the count or Nul-terminated length of the elements doesn't exceed max, and that no
// element includes the nul character.
func nullTerminatedByteCount(max uint32, elements []string) (uint32, error) {
	count := uint32(len(elements))
	if count > max {
		return 0, errors.New("exceeds maximum count")
	}

	// The buffer size is the total size including null terminators. The null terminator count == value count, sum
	// count with each value length. This works because in Go, the length of a string is the same as its byte count.
	bufSize, maxSize := uint64(count), uint64(max) // uint64 to allow summing without overflow
	for _, e := range elements {
		// As this is null-terminated, We have to validate there are no null characters in the string.
		for _, c := range e {
			if c == 0 {
				return 0, errors.New("contains NUL character")
			}
		}

		nextSize := bufSize + uint64(len(e))
		if nextSize > maxSize {
			return 0, errors.New("exceeds maximum size")
		}
		bufSize = nextSize

	}
	return uint32(bufSize), nil
}
