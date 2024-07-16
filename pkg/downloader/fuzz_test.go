package downloader

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
)

var a = digest.Algorithm("sha256")

func FuzzDownload(f *testing.F) {
	f.Fuzz(func(t *testing.T, fileContents []byte, checkDigest bool) {
		localFile := filepath.Join(t.TempDir(), "localFile")
		remoteFile := filepath.Join(t.TempDir(), "remoteFile")
		err := os.WriteFile(remoteFile, fileContents, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		testLocalFileURL := "file://" + remoteFile
		if checkDigest {
			d := a.FromBytes(fileContents)
			_, _ = Download(context.Background(),
				localFile,
				testLocalFileURL,
				WithExpectedDigest(d))
		} else {
			_, _ = Download(context.Background(),
				localFile,
				testLocalFileURL)
		}
	})
}
