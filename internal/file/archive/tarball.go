package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type TarGzWriter struct {
	*tar.Writer

	gw *gzip.Writer
}

func NewTarGzWriter(w io.Writer) *TarGzWriter {
	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	return &TarGzWriter{
		Writer: tw,
		gw:     gw,
	}
}

func (tgw *TarGzWriter) WriteFile(path string, bs []byte) (err error) {
	hdr := &tar.Header{
		Name:     path,
		Mode:     0600,
		Typeflag: tar.TypeReg,
		Size:     int64(len(bs)),
	}

	if err = tgw.WriteHeader(hdr); err == nil {
		_, err = tgw.Write(bs)
	}

	return err
}

func (tgw *TarGzWriter) WriteJSONFile(path string, v any) error {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(v); err != nil {
		return err
	}

	return tgw.WriteFile(path, buf.Bytes())
}

func (tgw *TarGzWriter) Close() error {
	return errors.Join(tgw.Writer.Close(), tgw.gw.Close())
}

// MustWriteTarGz writes the list of file names and content into a tarball.
// Paths are prefixed with "/".
func MustWriteTarGz(files [][2]string) *bytes.Buffer {
	buf := &bytes.Buffer{}
	tgw := NewTarGzWriter(buf)
	defer tgw.Close()

	for _, file := range files {
		if !strings.HasPrefix(file[0], "/") {
			file[0] = "/" + file[0]
		}

		if err := tgw.WriteFile(file[0], []byte(file[1])); err != nil {
			panic(err)
		}
	}

	return buf
}
