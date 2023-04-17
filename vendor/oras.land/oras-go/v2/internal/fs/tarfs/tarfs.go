/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tarfs

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"oras.land/oras-go/v2/errdef"
)

// blockSize is the size of each block in a tar archive.
const blockSize int64 = 512

// TarFS represents a file system (an fs.FS) based on a tar archive.
type TarFS struct {
	path    string
	entries map[string]*entry
}

// entry represents an entry in a tar archive.
type entry struct {
	header *tar.Header
	pos    int64
}

// New returns a file system (an fs.FS) for a tar archive located at path.
func New(path string) (*TarFS, error) {
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", path, err)
	}
	tarfs := &TarFS{
		path:    pathAbs,
		entries: make(map[string]*entry),
	}
	if err := tarfs.indexEntries(); err != nil {
		return nil, err
	}
	return tarfs, nil
}

// Open opens the named file.
// When Open returns an error, it should be of type *PathError
// with the Op field set to "open", the Path field set to name,
// and the Err field describing the problem.
//
// Open should reject attempts to open names that do not satisfy
// ValidPath(name), returning a *PathError with Err set to
// ErrInvalid or ErrNotExist.
func (tfs *TarFS) Open(name string) (file fs.File, openErr error) {
	entry, err := tfs.getEntry(name)
	if err != nil {
		return nil, err
	}
	tarFile, err := os.Open(tfs.path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if openErr != nil {
			tarFile.Close()
		}
	}()

	if _, err := tarFile.Seek(entry.pos, io.SeekStart); err != nil {
		return nil, err
	}
	tr := tar.NewReader(tarFile)
	if _, err := tr.Next(); err != nil {
		return nil, err
	}
	return &entryFile{
		Reader: tr,
		Closer: tarFile,
		header: entry.header,
	}, nil
}

// Stat returns a FileInfo describing the file.
// If there is an error, it should be of type *PathError.
func (tfs *TarFS) Stat(name string) (fs.FileInfo, error) {
	entry, err := tfs.getEntry(name)
	if err != nil {
		return nil, err
	}
	return entry.header.FileInfo(), nil
}

// getEntry returns the named entry.
func (tfs *TarFS) getEntry(name string) (*entry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Path: name, Err: fs.ErrInvalid}
	}
	entry, ok := tfs.entries[name]
	if !ok {
		return nil, &fs.PathError{Path: name, Err: fs.ErrNotExist}
	}
	if entry.header.Typeflag != tar.TypeReg {
		// support regular files only
		return nil, fmt.Errorf("%s: type flag %c is not supported: %w",
			name, entry.header.Typeflag, errdef.ErrUnsupported)
	}
	return entry, nil
}

// indexEntries index entries in the tar archive.
func (tfs *TarFS) indexEntries() error {
	tarFile, err := os.Open(tfs.path)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	tr := tar.NewReader(tarFile)
	for {
		header, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		pos, err := tarFile.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		tfs.entries[header.Name] = &entry{
			header: header,
			pos:    pos - blockSize,
		}
	}

	return nil
}

// entryFile represents an entryFile in a tar archive and implements `fs.File`.
type entryFile struct {
	io.Reader
	io.Closer
	header *tar.Header
}

// Stat returns a fs.FileInfo describing e.
func (e *entryFile) Stat() (fs.FileInfo, error) {
	return e.header.FileInfo(), nil
}
