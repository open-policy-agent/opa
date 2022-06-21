package sys

import (
	"context"
	"fmt"
	"io/fs"
	"math"
	"sync/atomic"
)

// FSKey is a context.Context Value key. It allows overriding fs.FS for WASI.
//
// See https://github.com/tetratelabs/wazero/issues/491
type FSKey struct{}

// FileEntry maps a path to an open file in a file system.
type FileEntry struct {
	Path string
	FS   fs.FS
	// File when nil this is a mount like "." or "/".
	File fs.File
}

type FSContext struct {
	// openedFiles is a map of file descriptor numbers (>=3) to open files (or directories) and defaults to empty.
	// TODO: This is unguarded, so not goroutine-safe!
	openedFiles map[uint32]*FileEntry

	// lastFD is not meant to be read directly. Rather by nextFD.
	lastFD uint32
}

func NewFSContext(openedFiles map[uint32]*FileEntry) *FSContext {
	var fsCtx FSContext
	if openedFiles == nil {
		fsCtx.openedFiles = map[uint32]*FileEntry{}
		fsCtx.lastFD = 2 // STDERR
	} else {
		fsCtx.openedFiles = openedFiles
		fsCtx.lastFD = 2 // STDERR
		for fd := range openedFiles {
			if fd > fsCtx.lastFD {
				fsCtx.lastFD = fd
			}
		}
	}
	return &fsCtx
}

// nextFD gets the next file descriptor number in a goroutine safe way (monotonically) or zero if we ran out.
// TODO: opendFiles is still not goroutine safe!
// TODO: This can return zero if we ran out of file descriptors. A future change can optimize by re-using an FD pool.
func (c *FSContext) nextFD() uint32 {
	if c.lastFD == math.MaxUint32 {
		return 0
	}
	return atomic.AddUint32(&c.lastFD, 1)
}

// Close implements io.Closer
func (c *FSContext) Close(_ context.Context) (err error) {
	// Close any files opened in this context
	for fd, entry := range c.openedFiles {
		delete(c.openedFiles, fd)
		if entry.File != nil { // File is nil for a mount like "." or "/"
			if e := entry.File.Close(); e != nil {
				err = e // This means the err returned == the last non-nil error.
			}
		}
	}
	return
}

// CloseFile returns true if a file was opened and closed without error, or false if not.
func (c *FSContext) CloseFile(fd uint32) (bool, error) {
	f, ok := c.openedFiles[fd]
	if !ok {
		return false, nil
	}
	delete(c.openedFiles, fd)

	if f.File == nil { // TODO: currently, this means it is a pre-opened filesystem, but this may change later.
		return true, nil
	}
	if err := f.File.Close(); err != nil {
		return false, err
	}
	return true, nil
}

// OpenedFile returns a file and true if it was opened or nil and false, if not.
func (c *FSContext) OpenedFile(fd uint32) (*FileEntry, bool) {
	f, ok := c.openedFiles[fd]
	return f, ok
}

// OpenFile returns the file descriptor of the new file or false if we ran out of file descriptors
func (c *FSContext) OpenFile(f *FileEntry) (uint32, bool) {
	newFD := c.nextFD()
	if newFD == 0 {
		return 0, false
	}
	c.openedFiles[newFD] = f
	return newFD, true
}

type FSConfig struct {
	// preopenFD has the next FD number to use
	preopenFD uint32
	// preopens are keyed on file descriptor and only include the Path and FS fields.
	preopens map[uint32]*FileEntry
	// preopenPaths allow overwriting of existing paths.
	preopenPaths map[string]uint32
}

func NewFSConfig() *FSConfig {
	return &FSConfig{
		preopenFD:    uint32(3), // after stdin/stdout/stderr
		preopens:     map[uint32]*FileEntry{},
		preopenPaths: map[string]uint32{},
	}
}

// Clone makes a deep copy of this FS config.
func (c *FSConfig) Clone() *FSConfig {
	ret := *c // copy except maps which share a ref
	ret.preopens = make(map[uint32]*FileEntry, len(c.preopens))
	for key, value := range c.preopens {
		ret.preopens[key] = value
	}
	ret.preopenPaths = make(map[string]uint32, len(c.preopenPaths))
	for key, value := range c.preopenPaths {
		ret.preopenPaths[key] = value
	}
	return &ret
}

// setFS maps a path to a file-system. This is only used for base paths: "/" and ".".
func (c *FSConfig) setFS(path string, fs fs.FS) {
	// Check to see if this key already exists and update it.
	entry := &FileEntry{Path: path, FS: fs}
	if fd, ok := c.preopenPaths[path]; ok {
		c.preopens[fd] = entry
	} else {
		c.preopens[c.preopenFD] = entry
		c.preopenPaths[path] = c.preopenFD
		c.preopenFD++
	}
}

func (c *FSConfig) WithFS(fs fs.FS) *FSConfig {
	ret := c.Clone()
	ret.setFS("/", fs)
	return ret
}

func (c *FSConfig) WithWorkDirFS(fs fs.FS) *FSConfig {
	ret := c.Clone()
	ret.setFS(".", fs)
	return ret
}

func (c *FSConfig) Preopens() (map[uint32]*FileEntry, error) {
	// Ensure no-one set a nil FD. We do this here instead of at the call site to allow chaining as nil is unexpected.
	rootFD := uint32(0) // zero is invalid
	setWorkDirFS := false
	preopens := c.preopens
	for fd, entry := range preopens {
		if entry.FS == nil {
			return nil, fmt.Errorf("FS for %s is nil", entry.Path)
		} else if entry.Path == "/" {
			rootFD = fd
		} else if entry.Path == "." {
			setWorkDirFS = true
		}
	}

	// Default the working directory to the root FS if it exists.
	if rootFD != 0 && !setWorkDirFS {
		preopens[c.preopenFD] = &FileEntry{Path: ".", FS: preopens[rootFD].FS}
	}

	return preopens, nil
}
