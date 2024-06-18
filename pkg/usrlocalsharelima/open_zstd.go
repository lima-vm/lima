//go:build !no_zstd

package usrlocalsharelima

import (
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

const Ext = ".zst"

func Open(path string) (io.ReadCloser, error) {
	reader, err := os.Open(path + Ext)
	if err != nil {
		return nil, err
	}
	decoder, err := zstd.NewReader(reader)
	if err != nil {
		return nil, err
	}
	return decoder.IOReadCloser(), nil
}
