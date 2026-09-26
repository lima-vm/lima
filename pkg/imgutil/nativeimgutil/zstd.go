// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package nativeimgutil

import (
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/lima-vm/go-qcow2reader/image/qcow2"
)

// go-qcow2reader only registers a decompressor for zlib by default.
func init() {
	qcow2.SetDecompressor(qcow2.CompressionTypeZstd, newZstdDecompressor)
}

type zstdDecompressor struct {
	*zstd.Decoder
}

func (d *zstdDecompressor) Close() error {
	d.Decoder.Close()
	return nil
}

func newZstdDecompressor(r io.Reader) (io.ReadCloser, error) {
	dec, err := zstd.NewReader(r)
	if err != nil {
		return nil, err
	}
	return &zstdDecompressor{dec}, nil
}
