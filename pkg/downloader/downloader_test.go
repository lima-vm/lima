// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package downloader

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
)

func TestMain(m *testing.M) {
	HideProgress = true
	m.Run()
}

type downloadResult struct {
	r   *Result
	err error
}

// We expect only few parallel downloads. Testing with larger number to find
// races quicker. 20 parallel downloads take about 120 milliseconds on M1 Pro.
const parallelDownloads = 20

func TestDownloadRemote(t *testing.T) {
	ts := httptest.NewServer(http.FileServer(http.Dir("testdata")))
	t.Cleanup(ts.Close)
	dummyRemoteFileURL := ts.URL + "/downloader.txt"
	const dummyRemoteFileDigest = "sha256:380481d26f897403368be7cb86ca03a4bc14b125bfaf2b93bff809a5a2ad717e"
	dummyRemoteFileStat, err := os.Stat(filepath.Join("testdata", "downloader.txt"))
	assert.NilError(t, err)

	t.Run("without cache", func(t *testing.T) {
		t.Run("without digest", func(t *testing.T) {
			ctx := t.Context()
			localPath := filepath.Join(t.TempDir(), t.Name())
			r, err := Download(ctx, localPath, dummyRemoteFileURL)
			assert.NilError(t, err)
			assert.Equal(t, StatusDownloaded, r.Status)

			// download again, make sure StatusSkippedIsReturned
			r, err = Download(ctx, localPath, dummyRemoteFileURL)
			assert.NilError(t, err)
			assert.Equal(t, StatusSkipped, r.Status)
		})
		t.Run("with digest", func(t *testing.T) {
			ctx := t.Context()
			wrongDigest := digest.Digest("sha256:8313944efb4f38570c689813f288058b674ea6c487017a5a4738dc674b65f9d9")
			localPath := filepath.Join(t.TempDir(), t.Name())
			_, err := Download(ctx, localPath, dummyRemoteFileURL, WithExpectedDigest(wrongDigest))
			assert.ErrorContains(t, err, "expected digest")

			wrongDigest2 := digest.Digest("8313944efb4f38570c689813f288058b674ea6c487017a5a4738dc674b65f9d9")
			_, err = Download(ctx, localPath, dummyRemoteFileURL, WithExpectedDigest(wrongDigest2))
			assert.ErrorContains(t, err, "invalid checksum digest format")

			r, err := Download(ctx, localPath, dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest))
			assert.NilError(t, err)
			assert.Equal(t, StatusDownloaded, r.Status)

			r, err = Download(ctx, localPath, dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest))
			assert.NilError(t, err)
			assert.Equal(t, StatusSkipped, r.Status)
		})
	})
	t.Run("with cache", func(t *testing.T) {
		t.Run("serial", func(t *testing.T) {
			ctx := t.Context()
			cacheDir := filepath.Join(t.TempDir(), "cache")
			localPath := filepath.Join(t.TempDir(), t.Name())
			r, err := Download(ctx, localPath, dummyRemoteFileURL,
				WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
			assert.NilError(t, err)
			assert.Equal(t, StatusDownloaded, r.Status)

			r, err = Download(ctx, localPath, dummyRemoteFileURL,
				WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
			assert.NilError(t, err)
			assert.Equal(t, StatusSkipped, r.Status)

			localPath2 := localPath + "-2"
			r, err = Download(ctx, localPath2, dummyRemoteFileURL,
				WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
			assert.NilError(t, err)
			assert.Equal(t, StatusUsedCache, r.Status)
		})
		t.Run("parallel", func(t *testing.T) {
			ctx := t.Context()
			cacheDir := filepath.Join(t.TempDir(), "cache")
			results := make(chan downloadResult, parallelDownloads)
			for range parallelDownloads {
				go func() {
					// Parallel download is supported only for different instances with unique localPath.
					localPath := filepath.Join(t.TempDir(), t.Name())
					r, err := Download(ctx, localPath, dummyRemoteFileURL,
						WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
					results <- downloadResult{r, err}
				}()
			}
			// Only one thread should download, the rest should use the cache.
			downloaded, cached := countResults(t, results)
			assert.Equal(t, downloaded, 1)
			assert.Equal(t, cached, parallelDownloads-1)
		})
	})
	t.Run("caching-only mode", func(t *testing.T) {
		t.Run("serial", func(t *testing.T) {
			ctx := t.Context()
			_, err := Download(ctx, "", dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest))
			assert.ErrorContains(t, err, "cache directory to be specified")

			cacheDir := filepath.Join(t.TempDir(), "cache")
			r, err := Download(ctx, "", dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest),
				WithCacheDir(cacheDir))
			assert.NilError(t, err)
			assert.Equal(t, StatusDownloaded, r.Status)

			r, err = Download(ctx, "", dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest),
				WithCacheDir(cacheDir))
			assert.NilError(t, err)
			assert.Equal(t, StatusUsedCache, r.Status)

			localPath := filepath.Join(t.TempDir(), t.Name())
			r, err = Download(ctx, localPath, dummyRemoteFileURL,
				WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
			assert.NilError(t, err)
			assert.Equal(t, StatusUsedCache, r.Status)
		})
		t.Run("parallel", func(t *testing.T) {
			ctx := t.Context()
			cacheDir := filepath.Join(t.TempDir(), "cache")
			results := make(chan downloadResult, parallelDownloads)
			for range parallelDownloads {
				go func() {
					r, err := Download(ctx, "", dummyRemoteFileURL,
						WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
					results <- downloadResult{r, err}
				}()
			}
			// Only one thread should download, the rest should use the cache.
			downloaded, cached := countResults(t, results)
			assert.Equal(t, downloaded, 1)
			assert.Equal(t, cached, parallelDownloads-1)
		})
	})
	t.Run("cached", func(t *testing.T) {
		ctx := t.Context()
		_, err := Cached(dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest))
		assert.ErrorContains(t, err, "cache directory to be specified")

		cacheDir := filepath.Join(t.TempDir(), "cache")
		r, err := Download(ctx, "", dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
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
		ctx := t.Context()
		_, err := Cached(dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest))
		assert.ErrorContains(t, err, "cache directory to be specified")

		cacheDir := filepath.Join(t.TempDir(), "cache")
		r, err := Download(ctx, "", dummyRemoteFileURL, WithExpectedDigest(dummyRemoteFileDigest), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
		assert.Equal(t, dummyRemoteFileStat.ModTime().Truncate(time.Second).UTC(), r.LastModified)
		assert.Equal(t, "text/plain; charset=utf-8", r.ContentType)
	})
}

func countResults(t *testing.T, results chan downloadResult) (downloaded, cached int) {
	t.Helper()
	for range parallelDownloads {
		result := <-results
		if result.err != nil {
			t.Errorf("Download failed: %s", result.err)
		} else {
			switch result.r.Status {
			case StatusDownloaded:
				downloaded++
			case StatusUsedCache:
				cached++
			default:
				t.Errorf("Unexpected download status %#q", result.r.Status)
			}
		}
	}
	return downloaded, cached
}

// seedPartial writes a partial download (data.part) and its cached Last-Modified
// into the cache entry for url, so a subsequent Download attempts to resume it.
func seedPartial(t *testing.T, cacheDir, url string, partial []byte, lastModified string) {
	t.Helper()
	shad := cacheDirectoryPath(cacheDir, url)
	assert.NilError(t, os.MkdirAll(shad, 0o700))
	assert.NilError(t, os.WriteFile(filepath.Join(shad, "data.part"), partial, 0o644))
	if lastModified != "" {
		assert.NilError(t, os.WriteFile(filepath.Join(shad, "time"), []byte(lastModified), 0o644))
	}
}

func TestResumeDownload(t *testing.T) {
	content := bytes.Repeat([]byte("lima-resume-test\n"), 100) // 1700 bytes
	dgst := digest.FromBytes(content)
	modtime := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	lastModified := modtime.UTC().Format(http.TimeFormat)
	const prefix = 400

	t.Run("resumes from partial", func(t *testing.T) {
		ctx := t.Context()
		var gotRange atomic.Bool
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Range") != "" {
				gotRange.Store(true)
			}
			// ServeContent handles Range/If-Range and replies 206 when the
			// If-Range validator (Last-Modified) matches modtime.
			http.ServeContent(w, r, "data", modtime, bytes.NewReader(content))
		}))
		t.Cleanup(ts.Close)

		cacheDir := filepath.Join(t.TempDir(), "cache")
		seedPartial(t, cacheDir, ts.URL, content[:prefix], lastModified)

		localPath := filepath.Join(t.TempDir(), "out")
		r, err := Download(ctx, localPath, ts.URL, WithExpectedDigest(dgst), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
		assert.Assert(t, gotRange.Load(), "server should have received a Range request")

		got, err := os.ReadFile(localPath)
		assert.NilError(t, err)
		assert.Assert(t, bytes.Equal(got, content), "resumed file must match the full content")
	})

	t.Run("falls back to full download when Range is ignored", func(t *testing.T) {
		ctx := t.Context()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// Always reply 200 with the full body, ignoring the Range header.
			w.Header().Set("Last-Modified", lastModified)
			_, _ = w.Write(content)
		}))
		t.Cleanup(ts.Close)

		cacheDir := filepath.Join(t.TempDir(), "cache")
		// Garbage partial must be discarded (O_TRUNC) on a 200 response.
		seedPartial(t, cacheDir, ts.URL, []byte("garbage-garbage-garbage"), lastModified)

		localPath := filepath.Join(t.TempDir(), "out")
		r, err := Download(ctx, localPath, ts.URL, WithExpectedDigest(dgst), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		got, err := os.ReadFile(localPath)
		assert.NilError(t, err)
		assert.Assert(t, bytes.Equal(got, content), "file must be the full content, not the garbage partial")
	})

	t.Run("restarts when remote changed (If-Range mismatch)", func(t *testing.T) {
		ctx := t.Context()
		// The remote is newer than the cached partial, so If-Range mismatches
		// and ServeContent replies 200 with the full (current) body.
		newModtime := time.Date(2021, time.June, 1, 0, 0, 0, 0, time.UTC)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(w, r, "data", newModtime, bytes.NewReader(content))
		}))
		t.Cleanup(ts.Close)

		cacheDir := filepath.Join(t.TempDir(), "cache")
		oldLastModified := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat)
		seedPartial(t, cacheDir, ts.URL, []byte("stale-stale-stale-stale"), oldLastModified)

		localPath := filepath.Join(t.TempDir(), "out")
		r, err := Download(ctx, localPath, ts.URL, WithExpectedDigest(dgst), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		got, err := os.ReadFile(localPath)
		assert.NilError(t, err)
		assert.Assert(t, bytes.Equal(got, content), "file must be the fresh full content")
	})

	t.Run("restarts on 416 when partial exceeds remote size", func(t *testing.T) {
		ctx := t.Context()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(w, r, "data", modtime, bytes.NewReader(content))
		}))
		t.Cleanup(ts.Close)

		cacheDir := filepath.Join(t.TempDir(), "cache")
		// Partial larger than the remote → Range unsatisfiable → 416 → restart.
		seedPartial(t, cacheDir, ts.URL, bytes.Repeat([]byte("x"), len(content)+500), lastModified)

		localPath := filepath.Join(t.TempDir(), "out")
		r, err := Download(ctx, localPath, ts.URL, WithExpectedDigest(dgst), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		got, err := os.ReadFile(localPath)
		assert.NilError(t, err)
		assert.Assert(t, bytes.Equal(got, content), "file must be the full content after 416 restart")
	})

	t.Run("finalizes a complete partial on 416 without re-downloading", func(t *testing.T) {
		ctx := t.Context()
		var requests atomic.Int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			http.ServeContent(w, r, "data", modtime, bytes.NewReader(content))
		}))
		t.Cleanup(ts.Close)

		cacheDir := filepath.Join(t.TempDir(), "cache")
		// The partial is already the complete file, so the Range request is
		// unsatisfiable (416) and it should be finalized in place.
		seedPartial(t, cacheDir, ts.URL, content, lastModified)

		localPath := filepath.Join(t.TempDir(), "out")
		r, err := Download(ctx, localPath, ts.URL, WithExpectedDigest(dgst), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
		assert.Equal(t, int32(1), requests.Load(), "a complete partial must be finalized without a second request")

		got, err := os.ReadFile(localPath)
		assert.NilError(t, err)
		assert.Assert(t, bytes.Equal(got, content), "finalized file must match the full content")
	})

	t.Run("restarts from scratch without a digest", func(t *testing.T) {
		ctx := t.Context()
		var gotRange atomic.Bool
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Range") != "" {
				gotRange.Store(true)
			}
			http.ServeContent(w, r, "data", modtime, bytes.NewReader(content))
		}))
		t.Cleanup(ts.Close)

		cacheDir := filepath.Join(t.TempDir(), "cache")
		// A valid partial exists, but without a digest its integrity cannot be
		// verified, so the download must restart (no Range request) rather than
		// trusting the partial.
		seedPartial(t, cacheDir, ts.URL, content[:prefix], lastModified)

		localPath := filepath.Join(t.TempDir(), "out")
		r, err := Download(ctx, localPath, ts.URL, WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
		assert.Assert(t, !gotRange.Load(), "server must not receive a Range request without a digest")

		got, err := os.ReadFile(localPath)
		assert.NilError(t, err)
		assert.Assert(t, bytes.Equal(got, content), "file must be the full content")
	})

	t.Run("discards corrupt partial on digest mismatch", func(t *testing.T) {
		ctx := t.Context()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(w, r, "data", modtime, bytes.NewReader(content))
		}))
		t.Cleanup(ts.Close)

		cacheDir := filepath.Join(t.TempDir(), "cache")
		// A correctly-sized but corrupt prefix: the resumed digest cannot match.
		corrupt := bytes.Repeat([]byte("?"), prefix)
		seedPartial(t, cacheDir, ts.URL, corrupt, lastModified)

		localPath := filepath.Join(t.TempDir(), "out")
		_, err := Download(ctx, localPath, ts.URL, WithExpectedDigest(dgst), WithCacheDir(cacheDir))
		assert.ErrorContains(t, err, "expected digest")

		// The corrupt partial must be removed so the next attempt can recover.
		part := filepath.Join(cacheDirectoryPath(cacheDir, ts.URL), "data.part")
		_, statErr := os.Stat(part)
		assert.Assert(t, os.IsNotExist(statErr), "corrupt data.part should have been removed")

		// The next attempt starts fresh and succeeds.
		r, err := Download(ctx, localPath, ts.URL, WithExpectedDigest(dgst), WithCacheDir(cacheDir))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
		got, err := os.ReadFile(localPath)
		assert.NilError(t, err)
		assert.Assert(t, bytes.Equal(got, content), "recovered file must match the full content")
	})
}

func TestRedownloadRemote(t *testing.T) {
	remoteDir := t.TempDir()
	ts := httptest.NewServer(http.FileServer(http.Dir(remoteDir)))
	t.Cleanup(ts.Close)

	downloadDir := t.TempDir()

	cacheOpt := WithCacheDir(t.TempDir())

	t.Run("digest-less", func(t *testing.T) {
		ctx := t.Context()
		remoteFile := filepath.Join(remoteDir, "digest-less.txt")
		assert.NilError(t, os.WriteFile(remoteFile, []byte("digest-less"), 0o644))
		assert.NilError(t, os.Chtimes(remoteFile, time.Now(), time.Now().Add(-time.Hour)))
		opt := []Opt{cacheOpt}

		// Download on the first call
		r, err := Download(ctx, filepath.Join(downloadDir, "1"), ts.URL+"/digest-less.txt", opt...)
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		// Next download will use the cached download
		r, err = Download(ctx, filepath.Join(downloadDir, "2"), ts.URL+"/digest-less.txt", opt...)
		assert.NilError(t, err)
		assert.Equal(t, StatusUsedCache, r.Status)

		// Modifying remote file will cause redownload
		assert.NilError(t, os.Chtimes(remoteFile, time.Now(), time.Now()))
		r, err = Download(ctx, filepath.Join(downloadDir, "3"), ts.URL+"/digest-less.txt", opt...)
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		// Next download will use the cached download
		r, err = Download(ctx, filepath.Join(downloadDir, "4"), ts.URL+"/digest-less.txt", opt...)
		assert.NilError(t, err)
		assert.Equal(t, StatusUsedCache, r.Status)
	})

	t.Run("has-digest", func(t *testing.T) {
		ctx := t.Context()
		remoteFile := filepath.Join(remoteDir, "has-digest.txt")
		bytes := []byte("has-digest")
		assert.NilError(t, os.WriteFile(remoteFile, bytes, 0o644))
		assert.NilError(t, os.Chtimes(remoteFile, time.Now(), time.Now().Add(-time.Hour)))

		digester := digest.SHA256.Digester()
		_, err := digester.Hash().Write(bytes)
		assert.NilError(t, err)
		opt := []Opt{cacheOpt, WithExpectedDigest(digester.Digest())}

		r, err := Download(ctx, filepath.Join(downloadDir, "has-digest1.txt"), ts.URL+"/has-digest.txt", opt...)
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
		r, err = Download(ctx, filepath.Join(downloadDir, "has-digest2.txt"), ts.URL+"/has-digest.txt", opt...)
		assert.NilError(t, err)
		assert.Equal(t, StatusUsedCache, r.Status)

		// modifying remote file won't cause redownload because expected digest is provided
		assert.NilError(t, os.Chtimes(remoteFile, time.Now(), time.Now()))
		r, err = Download(ctx, filepath.Join(downloadDir, "has-digest3.txt"), ts.URL+"/has-digest.txt", opt...)
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

		r, err := Download(t.Context(), localPath, testLocalFileURL)
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
	})

	t.Run("with file digest", func(t *testing.T) {
		ctx := t.Context()
		localPath := filepath.Join(t.TempDir(), t.Name())
		localTestFile := filepath.Join(t.TempDir(), "some-file")
		testDownloadFileContents := []byte("TestDownloadLocal")

		assert.NilError(t, os.WriteFile(localTestFile, testDownloadFileContents, 0o644))
		testLocalFileURL := "file://" + localTestFile
		wrongDigest := digest.Digest(emptyFileDigest)

		_, err := Download(ctx, localPath, testLocalFileURL, WithExpectedDigest(wrongDigest))
		assert.ErrorContains(t, err, "expected digest")

		r, err := Download(ctx, localPath, testLocalFileURL, WithExpectedDigest(testDownloadLocalDigest))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
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
	testContents := []byte("TestDownloadCompressed")

	// check builds a compressed fixture via write, downloads it with
	// decompression, and asserts the result round-trips. With noExternal set,
	// PATH is emptied so the built-in decoder runs instead of a host binary.
	check := func(t *testing.T, ext string, write func(io.Writer) error, noExternal bool) {
		t.Helper()
		if noExternal {
			t.Setenv("PATH", t.TempDir())
		}
		localPath := filepath.Join(t.TempDir(), t.Name())
		localFile := filepath.Join(t.TempDir(), "test-file"+ext)
		f, err := os.Create(localFile)
		assert.NilError(t, err)
		assert.NilError(t, write(f))
		assert.NilError(t, f.Close())

		r, err := Download(t.Context(), localPath, "file://"+localFile, WithDecompress(true))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)
		got, err := os.ReadFile(localPath)
		assert.NilError(t, err)
		assert.Equal(t, string(got), string(testContents))
	}
	writeGz := func(w io.Writer) error {
		gz := gzip.NewWriter(w)
		if _, err := gz.Write(testContents); err != nil {
			return err
		}
		return gz.Close()
	}

	t.Run("gzip", func(t *testing.T) { check(t, ".gz", writeGz, false) })
	t.Run("gzip without external binary", func(t *testing.T) { check(t, ".gz", writeGz, true) })

	t.Run("bzip2", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			// External bzip2 ships with MSYS2/Git for Windows but not
			// vanilla Windows, and there is no in-process equivalent
			// in the standard library.
			t.Skip("bzip2 binary required to build the fixture")
		}
		ctx := t.Context()
		localPath := filepath.Join(t.TempDir(), t.Name())
		localFile := filepath.Join(t.TempDir(), "test-file")
		testDownloadCompressedContents := []byte("TestDownloadCompressed")
		assert.NilError(t, os.WriteFile(localFile, testDownloadCompressedContents, 0o644))
		assert.NilError(t, exec.CommandContext(ctx, "bzip2", localFile).Run())
		localFile += ".bz2"
		testLocalFileURL := "file://" + localFile

		r, err := Download(ctx, localPath, testLocalFileURL, WithDecompress(true))
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

		r, err := Download(t.Context(), localPath, testLocalFileURL, WithDecompress(true))
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		got, err := os.ReadFile(localPath)
		assert.NilError(t, err)
		assert.Equal(t, string(got), string(testDownloadCompressedContents))
	})
}

// This test simulates the end-to-end flow of downloading a remote image and converting it.
func TestDownloadImageConversion(t *testing.T) {
	_, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skipf("qemu-img does not seem installed: %v", err)
	}

	remoteDir := t.TempDir()
	ts := httptest.NewServer(http.FileServer(http.Dir(remoteDir)))
	t.Cleanup(ts.Close)

	qcow2Path := filepath.Join(remoteDir, "test.qcow2")
	assert.NilError(t, exec.CommandContext(t.Context(), "qemu-img", "create", "-f", "qcow2", qcow2Path, "64K").Run())

	t.Run("without-digest", func(t *testing.T) {
		ctx := t.Context()
		cacheDir := t.TempDir()
		localPath := filepath.Join(t.TempDir(), "local")
		remoteURL := ts.URL + "/test.qcow2"

		// 1. First download, should convert to raw
		r, err := Download(ctx, localPath, remoteURL,
			WithCacheDir(cacheDir),
			WithDescription("image"),
			WithDecompress(true),
			WithImageFormats([]string{"raw"}),
		)
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		shad := cacheDirectoryPath(cacheDir, remoteURL)
		rawPath := filepath.Join(shad, "imgconv", "raw")
		_, err = os.Stat(rawPath)
		assert.NilError(t, err)

		// 2. Second download, should use cached raw
		localPath2 := filepath.Join(t.TempDir(), "local2")
		r, err = Download(ctx, localPath2, remoteURL,
			WithCacheDir(cacheDir),
			WithDescription("image"),
			WithDecompress(true),
			WithImageFormats([]string{"raw"}),
		)
		assert.NilError(t, err)
		assert.Equal(t, StatusUsedCache, r.Status)
		assert.Equal(t, rawPath, r.CachePath)
	})

	t.Run("with-digest", func(t *testing.T) {
		ctx := t.Context()
		cacheDir := t.TempDir()
		localPath := filepath.Join(t.TempDir(), "local")
		remoteURL := ts.URL + "/test.qcow2"

		content, err := os.ReadFile(qcow2Path)
		assert.NilError(t, err)
		originalDigest := digest.SHA256.FromBytes(content)

		// 1. First download, should convert to raw
		r, err := Download(ctx, localPath, remoteURL,
			WithCacheDir(cacheDir),
			WithDescription("image"),
			WithDecompress(true),
			WithImageFormats([]string{"raw"}),
			WithExpectedDigest(originalDigest),
		)
		assert.NilError(t, err)
		assert.Equal(t, StatusDownloaded, r.Status)

		shad := cacheDirectoryPath(cacheDir, remoteURL)
		rawPath := filepath.Join(shad, "imgconv", "raw")
		_, err = os.Stat(rawPath)
		assert.NilError(t, err)

		// 2. Second download, should use cached raw
		localPath2 := filepath.Join(t.TempDir(), "local2")
		r, err = Download(ctx, localPath2, remoteURL,
			WithCacheDir(cacheDir),
			WithDescription("image"),
			WithDecompress(true),
			WithImageFormats([]string{"raw"}),
			WithExpectedDigest(originalDigest),
		)
		assert.NilError(t, err)
		assert.Equal(t, StatusUsedCache, r.Status)
		assert.Equal(t, rawPath, r.CachePath)
	})
}

// This test focuses specifically on the scenario where the source image
// is already in the Lima cache but the converted version does not exist yet.
func TestDownloadImageConversionCached(t *testing.T) {
	_, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skipf("qemu-img does not seem installed: %v", err)
	}

	ctx := t.Context()
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	remoteURL := "https://example.com/test.qcow2"

	// Pre-populate cache with a qcow2 file
	shad := cacheDirectoryPath(cacheDir, remoteURL)
	assert.NilError(t, os.MkdirAll(shad, 0o700))
	shadData := filepath.Join(shad, "data")
	assert.NilError(t, exec.CommandContext(t.Context(), "qemu-img", "create", "-f", "qcow2", shadData, "64K").Run())

	// Provide a cached digest to avoid HTTP HEAD / re-download.
	content, err := os.ReadFile(shadData)
	assert.NilError(t, err)
	originalDigest := digest.SHA256.FromBytes(content)
	shadDigest := filepath.Join(shad, "sha256.digest")
	assert.NilError(t, os.WriteFile(shadDigest, []byte(originalDigest.String()), 0o644))

	localPath := filepath.Join(tmpDir, "local")

	// Call Download, it should find qcow2 in cache, see it's not supported, and convert it
	r, err := Download(ctx, localPath, remoteURL,
		WithCacheDir(cacheDir),
		WithDescription("image"),
		WithImageFormats([]string{"raw"}),
		WithExpectedDigest(originalDigest),
	)
	assert.NilError(t, err)
	assert.Equal(t, StatusUsedCache, r.Status)

	// Verify conversion happened
	rawPath := filepath.Join(shad, "imgconv", "raw")
	_, err = os.Stat(rawPath)
	assert.NilError(t, err)
	assert.Equal(t, rawPath, r.CachePath)
}
