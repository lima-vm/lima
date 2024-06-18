//go:build !no_gzip

package usrlocalsharelima

import (
	"io"
	"os"

	"compress/gzip"
)

const Ext = ".gz"

func Open(path string) (io.ReadCloser, error) {
	reader, err := os.Open(path + Ext)
	if err != nil {
		return nil, err
	}
	return gzip.NewReader(reader)
}
