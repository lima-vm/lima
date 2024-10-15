package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
)

func TestMain(m *testing.M) {
	HideProgress = true
	os.Exit(m.Run())
}

type downloadResult struct {
	r   *Result
	err error
}

// We expect only few parallel downloads. Testing with larger number to find
// races quicker. 20 parallel downloads take about 120 milliseocnds on M1 Pro.
const parallelDownloads = 20

// When downloading in parallel usually all downloads completed with
// StatusDownload, but some may be delayed and find the data file when they
// start. Can be reproduced locally using 100 parallel downloads.
var parallelStatus = []Status{StatusDownloaded, StatusUsedCache}

func TestDownloadRemote(t *testing.T) {
	ts := httptest.NewServer(http.FileServer(http.Dir("testdata")))
	t.Cleanup(ts.Close)
	dummyRemoteFileURL := ts.URL + "/downloader.txt"
	const dummyRemoteFileDigest = "sha256:380481d26f897403368be7cb86ca03a4bc14b125bfaf2b93bff809a5a2ad717e"
	dummyRemoteFileStat, err := os.Stat(filepath.Join("testdata", "downloader.txt"))
	assert.NilError(t, err)

	t.Run("without cache", func(t *testing.T) {
		t.Run("without digest", func(t *testing.T) {
			localPath := filepath.Join(t.TempDir(), t.Name())
			r, err := Download(context.Background(), localPath, dummyRemoteFileURL)
			assert.NilError(t, err)
			assert.Equal(t, StatusDownloaded, r.Status)

			// download again, make sure StatusSkippedIsReturned
			r, err = Download(context.Background(), localPath, dummyRemoteFileURL)
			assert.NilError(t, err)
			assert.Equal(t, StatusSkipped, r.Status)
		})
		t.Run("with digest", func(t *testing.T) {
			wrongDigest := digest.Digest("sha256:8313944efb4f38570c689813f288058b674ea6c487017a5a4738dc674b65f9d9")
			localPath := filepath.Join(t.TempDir(), t.Name())
			_, err := Download(context.Background(), localPath, dummyRemoteFileURL, WithExpectedDigest(wrongDigest))
			assert.ErrorContains(t, err, "expected digest")

			r, err := Download(context.Background(), localPath, dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest))
			assert.NilError(t, err)
			assert.Equal(t, StatusDownloaded, r.Status)

			r, err = Download(context.Background(), localPath, dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest))
			assert.NilError(t, err)
			assert.Equal(t, StatusSkipped, r.Status)
		})
	})
	t.Run("with cache", func(t *testing.T) {
		t.Run("serial", func(t *testing.T) {
			cacheDir := filepath.Join(t.TempDir(), "cache")
			localPath := filepath.Join(t.TempDir(), t.Name())
			r, err := Download(context.Background(), localPath, dummyRemoteFileURL,
				WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
			assert.NilError(t, err)
			assert.Equal(t, StatusDownloaded, r.Status)

			r, err = Download(context.Background(), localPath, dummyRemoteFileURL,
				WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
			assert.NilError(t, err)
			assert.Equal(t, StatusSkipped, r.Status)

			localPath2 := localPath + "-2"
			r, err = Download(context.Background(), localPath2, dummyRemoteFileURL,
				WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
			assert.NilError(t, err)
			assert.Equal(t, StatusUsedCache, r.Status)
		})
		t.Run("parallel", func(t *testing.T) {
			cacheDir := filepath.Join(t.TempDir(), "cache")
			results := make(chan downloadResult, parallelDownloads)
			for i := 0; i < parallelDownloads; i++ {
				go func() {
					// Parallel download is supported only for different instances with unique localPath.
					localPath := filepath.Join(t.TempDir(), t.Name())
					r, err := Download(context.Background(), localPath, dummyRemoteFileURL,
						WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
					results <- downloadResult{r, err}
				}()
			}
			// We must process all results before cleanup.
			for i := 0; i < parallelDownloads; i++ {
				result := <-results
				if result.err != nil {
					t.Errorf("Download failed: %s", result.err)
				} else if !slices.Contains(parallelStatus, result.r.Status) {
					t.Errorf("Expected download status %s, got %s", parallelStatus, result.r.Status)
				}
			}
		})
	})
	t.Run("caching-only mode", func(t *testing.T) {
		t.Run("serial", func(t *testing.T) {
			_, err := Download(context.Background(), "", dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest))
			assert.ErrorContains(t, err, "cache directory to be specified")

			cacheDir := filepath.Join(t.TempDir(), "cache")
			r, err := Download(context.Background(), "", dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest),
				WithCacheDir(cacheDir))
			assert.NilError(t, err)
			assert.Equal(t, StatusDownloaded, r.Status)

			r, err = Download(context.Background(), "", dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest),
				WithCacheDir(cacheDir))
			assert.NilError(t, err)
			assert.Equal(t, StatusUsedCache, r.Status)

			localPath := filepath.Join(t.TempDir(), t.Name())
			r, err = Download(context.Background(), localPath, dummyRemoteFileURL,
				WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
			assert.NilError(t, err)
			assert.Equal(t, StatusUsedCache, r.Status)
		})
		t.Run("parallel", func(t *testing.T) {
			cacheDir := filepath.Join(t.TempDir(), "cache")
			results := make(chan downloadResult, parallelDownloads)
			for i := 0; i < parallelDownloads; i++ {
				go func() {
					r, err := Download(context.Background(), "", dummyRemoteFileURL,
						WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
					results <- downloadResult{r, err}
				}()
			}
			// We must process all results before cleanup.
			for i := 0; i < parallelDownloads; i++ {
				result := <-results
				if result.err != nil {
					t.Errorf("Download failed: %s", result.err)
				} else if !slices.Contains(parallelStatus, result.r.Status) {
					t.Errorf("Expected download status %s, got %s", parallelStatus, result.r.Status)
				}
			}
		})
	})
	t.Run("cached", func(t *testing.T) {
		_, err := Cached(dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest))
		assert.ErrorContains(t, err, "cache directory to be specified")

		cacheDir := filepath.Join(t.TempDir(), "cache")
		r, err := Download(context.Background(), "", dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		r, err = Cached(dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusUsedCache, r.Status)
		assert.Assert(t, strings.HasPrefix(r.CachePath, cacheDir), "expected %s to be in %s", r.CachePath, cacheDir)

		wrongDigest := digest.Digest("sha256:8313944efb4f38570c689813f288058b674ea6c487017a5a4738dc674b65f9d9")
		_, err = Cached(dummyRemoteFileURL, WithExpectedDigest(wrongDigest), WithCacheDir(cacheDir))
		assert.ErrorContains(t, err, "expected digest")
	})
	t.Run("metadata", func(t *testing.T) {
		_, err := Cached(dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest))
		assert.ErrorContains(t, err, "cache directory to be specified")

		cacheDir := filepath.Join(t.TempDir(), "cache")
		r, err := Download(context.Background(), "", dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
		assert.Equal(t, dummyRemoteFileStat.ModTime().Truncate(time.Second).UTC(), r.LastModified)
		assert.Equal(t, "text/plain; charset=utf-8", r.ContentType)
	})
}

func TestRedownloadRemote(t *testing.T) {
	remoteDir, err := os.MkdirTemp("", "redownloadRemote")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(remoteDir) })
	ts := httptest.NewServer(http.FileServer(http.Dir(remoteDir)))
	t.Cleanup(ts.Close)

	cacheDir, err := os.MkdirTemp("", "redownloadCache")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(cacheDir) })

	downloadDir, err := os.MkdirTemp("", "redownloadLocal")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(downloadDir) })

	cacheOpt := WithCacheDir(cacheDir)

	t.Run("digest-less", func(t *testing.T) {
		remoteFile := filepath.Join(remoteDir, "digest-less.txt")
		assert.NilError(t, os.WriteFile(remoteFile, []byte("digest-less"), 0o644))
		assert.NilError(t, os.Chtimes(remoteFile, time.Now(), time.Now().Add(-time.Hour)))
		opt := []Opt{cacheOpt}

		r, err := Download(context.Background(), filepath.Join(downloadDir, "digest-less1.txt"), ts.URL+"/digest-less.txt", opt...)
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
		r, err = Download(context.Background(), filepath.Join(downloadDir, "digest-less2.txt"), ts.URL+"/digest-less.txt", opt...)
		assert.NilError(t, err)
		assert.Equal(t, StatusUsedCache, r.Status)

		// modifying remote file will cause redownload
		assert.NilError(t, os.Chtimes(remoteFile, time.Now(), time.Now()))
		r, err = Download(context.Background(), filepath.Join(downloadDir, "digest-less3.txt"), ts.URL+"/digest-less.txt", opt...)
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
	})

	t.Run("has-digest", func(t *testing.T) {
		remoteFile := filepath.Join(remoteDir, "has-digest.txt")
		bytes := []byte("has-digest")
		assert.NilError(t, os.WriteFile(remoteFile, bytes, 0o644))
		assert.NilError(t, os.Chtimes(remoteFile, time.Now(), time.Now().Add(-time.Hour)))

		digester := digest.SHA256.Digester()
		_, err := digester.Hash().Write(bytes)
		assert.NilError(t, err)
		opt := []Opt{cacheOpt, WithExpectedDigest(digester.Digest())}

		r, err := Download(context.Background(), filepath.Join(downloadDir, "has-digest1.txt"), ts.URL+"/has-digest.txt", opt...)
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
		r, err = Download(context.Background(), filepath.Join(downloadDir, "has-digest2.txt"), ts.URL+"/has-digest.txt", opt...)
		assert.NilError(t, err)
		assert.Equal(t, StatusUsedCache, r.Status)

		// modifying remote file won't cause redownload because expected digest is provided
		assert.NilError(t, os.Chtimes(remoteFile, time.Now(), time.Now()))
		r, err = Download(context.Background(), filepath.Join(downloadDir, "has-digest3.txt"), ts.URL+"/has-digest.txt", opt...)
		assert.NilError(t, err)
		assert.Equal(t, StatusUsedCache, r.Status)
	})
}

func TestDownloadLocal(t *testing.T) {
	const emptyFileDigest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	const testDownloadLocalDigest = "sha256:0c1e0fba69e8919b306d030bf491e3e0c46cf0a8140ff5d7516ba3a83cbea5b3"

	t.Run("without digest", func(t *testing.T) {
		localPath := filepath.Join(t.TempDir(), t.Name())
		localFile := filepath.Join(t.TempDir(), "test-file")
		f, err := os.Create(localFile)
		assert.NilError(t, err)
		t.Cleanup(func() { _ = f.Close() })
		testLocalFileURL := "file://" + localFile

		r, err := Download(context.Background(), localPath, testLocalFileURL)
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
	})

	t.Run("with file digest", func(t *testing.T) {
		localPath := filepath.Join(t.TempDir(), t.Name())
		localTestFile := filepath.Join(t.TempDir(), "some-file")
		testDownloadFileContents := []byte("TestDownloadLocal")

		assert.NilError(t, os.WriteFile(localTestFile, testDownloadFileContents, 0o644))
		testLocalFileURL := "file://" + localTestFile
		wrongDigest := digest.Digest(emptyFileDigest)

		_, err := Download(context.Background(), localPath, testLocalFileURL, WithExpectedDigest(wrongDigest))
		assert.ErrorContains(t, err, "expected digest")

		r, err := Download(context.Background(), localPath, testLocalFileURL, WithExpectedDigest(testDownloadLocalDigest))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		os.Remove(localTestFile)
	})

	t.Run("cached", func(t *testing.T) {
		localFile := filepath.Join(t.TempDir(), "test-file")
		f, err := os.Create(localFile)
		assert.NilError(t, err)
		t.Cleanup(func() { _ = f.Close() })
		testLocalFileURL := "file://" + localFile

		cacheDir := filepath.Join(t.TempDir(), "cache")
		_, err = Cached(testLocalFileURL, WithCacheDir(cacheDir))
		assert.ErrorContains(t, err, "not cached")
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
		assert.NilError(t, os.WriteFile(localFile, testDownloadCompressedContents, 0o644))
		assert.NilError(t, exec.Command("gzip", localFile).Run())
		localFile += ".gz"
		testLocalFileURL := "file://" + localFile

		r, err := Download(context.Background(), localPath, testLocalFileURL, WithDecompress(true))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		got, err := os.ReadFile(localPath)
		assert.NilError(t, err)
		assert.Equal(t, string(got), string(testDownloadCompressedContents))
	})

	t.Run("bzip2", func(t *testing.T) {
		localPath := filepath.Join(t.TempDir(), t.Name())
		localFile := filepath.Join(t.TempDir(), "test-file")
		testDownloadCompressedContents := []byte("TestDownloadCompressed")
		assert.NilError(t, os.WriteFile(localFile, testDownloadCompressedContents, 0o644))
		assert.NilError(t, exec.Command("bzip2", localFile).Run())
		localFile += ".bz2"
		testLocalFileURL := "file://" + localFile

		r, err := Download(context.Background(), localPath, testLocalFileURL, WithDecompress(true))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		got, err := os.ReadFile(localPath)
		assert.NilError(t, err)
		assert.Equal(t, string(got), string(testDownloadCompressedContents))
	})

	t.Run("unknown decompressor", func(t *testing.T) {
		localPath := filepath.Join(t.TempDir(), t.Name())
		localFile := filepath.Join(t.TempDir(), "test-file.rar")
		testDownloadCompressedContents := []byte("TestDownloadCompressed")
		assert.NilError(t, os.WriteFile(localFile, testDownloadCompressedContents, 0o644))
		testLocalFileURL := "file://" + localFile

		r, err := Download(context.Background(), localPath, testLocalFileURL, WithDecompress(true))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		got, err := os.ReadFile(localPath)
		assert.NilError(t, err)
		assert.Equal(t, string(got), string(testDownloadCompressedContents))
	})
}
