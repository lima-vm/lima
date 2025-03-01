// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package downloader

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
)

var algorithm = digest.Algorithm("sha256")

func FuzzDownload(f *testing.F) {
	f.Fuzz(func(t *testing.T, fileContents []byte, checkDigest bool) {
		localFile := filepath.Join(t.TempDir(), "localFile")
		remoteFile := filepath.Join(t.TempDir(), "remoteFile")
		err := os.WriteFile(remoteFile, fileContents, 0o600)
		assert.NilError(t, err)
		testLocalFileURL := "file://" + remoteFile
		if checkDigest {
			d := algorithm.FromBytes(fileContents)
			_, _ = Download(context.Background(), localFile, testLocalFileURL, WithExpectedDigest(d))
		} else {
			_, _ = Download(context.Background(), localFile, testLocalFileURL)
		}
	})
}
