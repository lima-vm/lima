/*
Copyright The Lima Authors.

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
