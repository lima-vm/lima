package downloader

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
)

// TODO: create a localhost HTTP server to serve the test contents without Internet
const (
	dummyRemoteFileURL    = "https://raw.githubusercontent.com/lima-vm/lima/7459f4587987ed014c372f17b82de1817feffa2e/README.md"
	dummyRemoteFileDigest = "sha256:58d2de96f9d91f0acd93cb1e28bf7c42fc86079037768d6aa63b4e7e7b3c9be0"
)

func TestMain(m *testing.M) {
	HideProgress = true
	os.Exit(m.Run())
}

func TestDownloadRemote(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Run("without cache", func(t *testing.T) {
		t.Run("without digest", func(t *testing.T) {
			localPath := filepath.Join(t.TempDir(), t.Name())
			r, err := Download(localPath, dummyRemoteFileURL)
			assert.NilError(t, err)
			assert.Equal(t, StatusDownloaded, r.Status)

			// download again, make sure StatusSkippedIsReturned
			r, err = Download(localPath, dummyRemoteFileURL)
			assert.NilError(t, err)
			assert.Equal(t, StatusSkipped, r.Status)
		})
		t.Run("with digest", func(t *testing.T) {
			wrongDigest := digest.Digest("sha256:8313944efb4f38570c689813f288058b674ea6c487017a5a4738dc674b65f9d9")
			localPath := filepath.Join(t.TempDir(), t.Name())
			_, err := Download(localPath, dummyRemoteFileURL, WithExpectedDigest(wrongDigest))
			assert.ErrorContains(t, err, "expected digest")

			r, err := Download(localPath, dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest))
			assert.NilError(t, err)
			assert.Equal(t, StatusDownloaded, r.Status)

			r, err = Download(localPath, dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest))
			assert.NilError(t, err)
			assert.Equal(t, StatusSkipped, r.Status)
		})
	})
	t.Run("with cache", func(t *testing.T) {
		cacheDir := filepath.Join(t.TempDir(), "cache")
		localPath := filepath.Join(t.TempDir(), t.Name())
		r, err := Download(localPath, dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		r, err = Download(localPath, dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusSkipped, r.Status)

		localPath2 := localPath + "-2"
		r, err = Download(localPath2, dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusUsedCache, r.Status)
	})
	t.Run("caching-only mode", func(t *testing.T) {
		_, err := Download("", dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest))
		assert.ErrorContains(t, err, "cache directory to be specified")

		cacheDir := filepath.Join(t.TempDir(), "cache")
		r, err := Download("", dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		r, err = Download("", dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusUsedCache, r.Status)

		localPath := filepath.Join(t.TempDir(), t.Name())
		r, err = Download(localPath, dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusUsedCache, r.Status)
	})
}

func TestDownloadLocal(t *testing.T) {

	if runtime.GOOS == "windows" {
		// FIXME: `TempDir RemoveAll cleanup: remove C:\users\runner\Temp\TestDownloadLocalwithout_digest2738386858\002\test-file: Sharing violation.`
		t.Skip("Skipping on windows")
	}

	const emptyFileDigest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	const testDownloadLocalDigest = "sha256:0c1e0fba69e8919b306d030bf491e3e0c46cf0a8140ff5d7516ba3a83cbea5b3"

	t.Run("without digest", func(t *testing.T) {
		localPath := filepath.Join(t.TempDir(), t.Name())
		localFile := filepath.Join(t.TempDir(), "test-file")
		os.Create(localFile)
		testLocalFileURL := "file://" + localFile

		r, err := Download(localPath, testLocalFileURL)
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
	})

	t.Run("with file digest", func(t *testing.T) {
		localPath := filepath.Join(t.TempDir(), t.Name())
		localTestFile := filepath.Join(t.TempDir(), "some-file")
		testDownloadFileContents := []byte("TestDownloadLocal")

		assert.NilError(t, os.WriteFile(localTestFile, testDownloadFileContents, 0644))
		testLocalFileURL := "file://" + localTestFile
		wrongDigest := digest.Digest(emptyFileDigest)

		_, err := Download(localPath, testLocalFileURL, WithExpectedDigest(wrongDigest))
		assert.ErrorContains(t, err, "expected digest")

		r, err := Download(localPath, testLocalFileURL, WithExpectedDigest(testDownloadLocalDigest))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		os.Remove(localTestFile)
	})

}

func TestDownloadCompressed(t *testing.T) {

	if runtime.GOOS == "windows" {
		// FIXME: `assertion failed: error is not nil: exec: "gzip": executable file not found in %PATH%`
		t.Skip("Skipping on windows")
	}

	t.Run("gzip", func(t *testing.T) {
		localPath := filepath.Join(t.TempDir(), t.Name())
		localFile := filepath.Join(t.TempDir(), "test-file")
		testDownloadCompressedContents := []byte("TestDownloadCompressed")
		assert.NilError(t, os.WriteFile(localFile, testDownloadCompressedContents, 0644))
		assert.NilError(t, exec.Command("gzip", localFile).Run())
		localFile += ".gz"
		testLocalFileURL := "file://" + localFile

		r, err := Download(localPath, testLocalFileURL, WithDecompress(true))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		got, err := os.ReadFile(localPath)
		assert.NilError(t, err)
		assert.Equal(t, string(got), string(testDownloadCompressedContents))
	})
}
