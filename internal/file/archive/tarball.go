package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"
	"time"
)

// Writer allows writing file entries into a tarball.
type Writer struct {
	tw      *tar.Writer
	modTime time.Time
}

// New returns a Writer for writing tar file entries into the passed
// tar.Writer. All the file headers will have a mod time equal to the
// time New() was called.
func New(tw *tar.Writer) *Writer {
	return &Writer{
		tw:      tw,
		modTime: time.Now(),
	}
}

// MustWriteTarGz write the list of file names and content
// into a tarball.
func MustWriteTarGz(files [][2]string) *bytes.Buffer {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	ar := New(tw)
	defer tw.Close()
	for _, file := range files {
		if err := ar.WriteFile(file[0], []byte(file[1])); err != nil {
			panic(err)
		}
	}
	return &buf
}

// WriteFile adds a file header with content to the given tar writer
func (ar *Writer) WriteFile(path string, bs []byte) error {

	hdr := &tar.Header{
		Name:     "/" + strings.TrimLeft(path, "/"),
		Mode:     0600,
		Typeflag: tar.TypeReg,
		Size:     int64(len(bs)),
		ModTime:  ar.modTime,
	}

	if err := ar.tw.WriteHeader(hdr); err != nil {
		return err
	}

	_, err := ar.tw.Write(bs)
	return err
}
