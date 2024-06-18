//go:build no_zstd

package usrlocalsharelima

import (
	"io"
	"os"
)

const Ext = ""

func Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}
