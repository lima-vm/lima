package downloader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/containerd/continuity/fs"
	"github.com/lima-vm/lima/pkg/httpclientutil"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/lima/pkg/lockutil"
	"github.com/lima-vm/lima/pkg/progressbar"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

// HideProgress is used only for testing.
var HideProgress bool

// hideBar is used only for testing.
func hideBar(bar *progressbar.ProgressBar) {
	bar.Set(pb.Static, true)
}

type Status = string

const (
	StatusUnknown    Status = ""
	StatusDownloaded Status = "downloaded"
	StatusSkipped    Status = "skipped"
	StatusUsedCache  Status = "used-cache"
)

type Result struct {
	Status          Status
	CachePath       string // "/Users/foo/Library/Caches/lima/download/by-url-sha256/<SHA256_OF_URL>/data"
	LastModified    time.Time
	ContentType     string
	ValidatedDigest bool
}

type options struct {
	cacheDir       string // default: empty (disables caching)
	decompress     bool   // default: false (keep compression)
	description    string // default: url
	expectedDigest digest.Digest
}

func (o *options) apply(opts []Opt) error {
	for _, f := range opts {
		if err := f(o); err != nil {
			return err
		}
	}
	return nil
}

type Opt func(*options) error

// WithCache enables caching using filepath.Join(os.UserCacheDir(), "lima") as the cache dir.
func WithCache() Opt {
	return func(o *options) error {
		ucd, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		cacheDir := filepath.Join(ucd, "lima")
		return WithCacheDir(cacheDir)(o)
	}
}

// WithCacheDir enables caching using the specified dir.
// Empty value disables caching.
func WithCacheDir(cacheDir string) Opt {
	return func(o *options) error {
		o.cacheDir = cacheDir
		return nil
	}
}

// WithDescription adds a user description of the download.
func WithDescription(description string) Opt {
	return func(o *options) error {
		o.description = description
		return nil
	}
}

// WithDecompress decompress the download from the cache.
func WithDecompress(decompress bool) Opt {
	return func(o *options) error {
		o.decompress = decompress
		return nil
	}
}

// WithExpectedDigest is used to validate the downloaded file against the expected digest.
//
// The digest is not verified in the following cases:
//   - The digest was not specified.
//   - The file already exists in the local target path.
//
// When the `data` file exists in the cache dir with `<ALGO>.digest` file,
// the digest is verified by comparing the content of `<ALGO>.digest` with the expected
// digest string. So, the actual digest of the `data` file is not computed.
func WithExpectedDigest(expectedDigest digest.Digest) Opt {
	return func(o *options) error {
		if expectedDigest != "" {
			if !expectedDigest.Algorithm().Available() {
				return fmt.Errorf("expected digest algorithm %q is not available", expectedDigest.Algorithm())
			}
			if err := expectedDigest.Validate(); err != nil {
				return err
			}
		}

		o.expectedDigest = expectedDigest
		return nil
	}
}

func readFile(path string) string {
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func readTime(path string) time.Time {
	if path == "" {
		return time.Time{}
	}
	if _, err := os.Stat(path); err != nil {
		return time.Time{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(http.TimeFormat, string(b))
	if err != nil {
		return time.Time{}
	}
	return t
}

// Download downloads the remote resource into the local path.
//
// Download caches the remote resource if WithCache or WithCacheDir option is specified.
// Local files are not cached.
//
// When the local path already exists, Download returns Result with StatusSkipped.
// (So, the local path cannot be set to /dev/null for "caching only" mode.)
//
// The local path can be an empty string for "caching only" mode.
func Download(ctx context.Context, local, remote string, opts ...Opt) (*Result, error) {
	var o options
	if err := o.apply(opts); err != nil {
		return nil, err
	}

	var localPath string
	if local == "" {
		if o.cacheDir == "" {
			return nil, fmt.Errorf("caching-only mode requires the cache directory to be specified")
		}
	} else {
		var err error
		localPath, err = canonicalLocalPath(local)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(localPath); err == nil {
			logrus.Debugf("file %q already exists, skipping downloading from %q (and skipping digest validation)", localPath, remote)
			res := &Result{
				Status:          StatusSkipped,
				ValidatedDigest: false,
			}
			return res, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}

		localPathDir := filepath.Dir(localPath)
		if err := os.MkdirAll(localPathDir, 0o755); err != nil {
			return nil, err
		}
	}

	ext := path.Ext(remote)
	if IsLocal(remote) {
		if err := copyLocal(ctx, localPath, remote, ext, o.decompress, o.description, o.expectedDigest); err != nil {
			return nil, err
		}
		res := &Result{
			Status:          StatusDownloaded,
			ValidatedDigest: o.expectedDigest != "",
		}
		return res, nil
	}

	if o.cacheDir == "" {
		if err := downloadHTTP(ctx, localPath, "", "", remote, o.description, o.expectedDigest); err != nil {
			return nil, err
		}
		res := &Result{
			Status:          StatusDownloaded,
			ValidatedDigest: o.expectedDigest != "",
		}
		return res, nil
	}

	shad := cacheDirectoryPath(o.cacheDir, remote)
	if err := os.MkdirAll(shad, 0o700); err != nil {
		return nil, err
	}

	var res *Result
	err := lockutil.WithDirLock(shad, func() error {
		var err error
		res, err = getCached(ctx, localPath, remote, o)
		if err != nil {
			return err
		}
		if res != nil {
			return nil
		}
		res, err = fetch(ctx, localPath, remote, o)
		return err
	})
	return res, err
}

// getCached tries to copy the file from the cache to local path. Return result,
// nil if the file was copied, nil, nil if the file is not in the cache or the
// cache needs update, or nil, error on fatal error.
func getCached(ctx context.Context, localPath, remote string, o options) (*Result, error) {
	shad := cacheDirectoryPath(o.cacheDir, remote)
	shadData := filepath.Join(shad, "data")
	shadTime := filepath.Join(shad, "time")
	shadType := filepath.Join(shad, "type")
	shadDigest, err := cacheDigestPath(shad, o.expectedDigest)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(shadData); err != nil {
		return nil, nil
	}
	ext := path.Ext(remote)
	logrus.Debugf("file %q is cached as %q", localPath, shadData)
	if _, err := os.Stat(shadDigest); err == nil {
		logrus.Debugf("Comparing digest %q with the cached digest file %q, not computing the actual digest of %q",
			o.expectedDigest, shadDigest, shadData)
		if err := validateCachedDigest(shadDigest, o.expectedDigest); err != nil {
			return nil, err
		}
		if err := copyLocal(ctx, localPath, shadData, ext, o.decompress, "", ""); err != nil {
			return nil, err
		}
	} else {
		if match, lmCached, lmRemote, err := matchLastModified(ctx, shadTime, remote); err != nil {
			logrus.WithError(err).Info("Failed to retrieve last-modified for cached digest-less image; using cached image.")
		} else if match {
			if err := copyLocal(ctx, localPath, shadData, ext, o.decompress, o.description, o.expectedDigest); err != nil {
				return nil, err
			}
		} else {
			logrus.Infof("Re-downloading digest-less image: last-modified mismatch (cached: %q, remote: %q)", lmCached, lmRemote)
			return nil, nil
		}
	}
	res := &Result{
		Status:          StatusUsedCache,
		CachePath:       shadData,
		LastModified:    readTime(shadTime),
		ContentType:     readFile(shadType),
		ValidatedDigest: o.expectedDigest != "",
	}
	return res, nil
}

// fetch downloads remote to the cache and copy the cached file to local path.
func fetch(ctx context.Context, localPath, remote string, o options) (*Result, error) {
	shad := cacheDirectoryPath(o.cacheDir, remote)
	shadData := filepath.Join(shad, "data")
	shadTime := filepath.Join(shad, "time")
	shadType := filepath.Join(shad, "type")
	shadDigest, err := cacheDigestPath(shad, o.expectedDigest)
	if err != nil {
		return nil, err
	}
	ext := path.Ext(remote)
	shadURL := filepath.Join(shad, "url")
	if err := os.WriteFile(shadURL, []byte(remote), 0o644); err != nil {
		return nil, err
	}
	if err := downloadHTTP(ctx, shadData, shadTime, shadType, remote, o.description, o.expectedDigest); err != nil {
		return nil, err
	}
	if shadDigest != "" && o.expectedDigest != "" {
		if err := os.WriteFile(shadDigest, []byte(o.expectedDigest.String()), 0o644); err != nil {
			return nil, err
		}
	}
	// no need to pass the digest to copyLocal(), as we already verified the digest
	if err := copyLocal(ctx, localPath, shadData, ext, o.decompress, "", ""); err != nil {
		return nil, err
	}
	res := &Result{
		Status:          StatusDownloaded,
		CachePath:       shadData,
		LastModified:    readTime(shadTime),
		ContentType:     readFile(shadType),
		ValidatedDigest: o.expectedDigest != "",
	}
	return res, nil
}

// Cached checks if the remote resource is in the cache.
//
// Download caches the remote resource if WithCache or WithCacheDir option is specified.
// Local files are not cached.
//
// When the cache path already exists, Cached returns Result with StatusUsedCache.
func Cached(remote string, opts ...Opt) (*Result, error) {
	var o options
	if err := o.apply(opts); err != nil {
		return nil, err
	}
	if o.cacheDir == "" {
		return nil, fmt.Errorf("caching-only mode requires the cache directory to be specified")
	}
	if IsLocal(remote) {
		return nil, fmt.Errorf("local files are not cached")
	}

	shad := cacheDirectoryPath(o.cacheDir, remote)
	shadData := filepath.Join(shad, "data")
	shadTime := filepath.Join(shad, "time")
	shadType := filepath.Join(shad, "type")
	shadDigest, err := cacheDigestPath(shad, o.expectedDigest)
	if err != nil {
		return nil, err
	}

	// Checking if data file exists is safe without locking.
	if _, err := os.Stat(shadData); err != nil {
		return nil, err
	}

	// But validating the digest or the data file must take the lock to avoid races
	// with parallel downloads.
	if err := os.MkdirAll(shad, 0o700); err != nil {
		return nil, err
	}
	err = lockutil.WithDirLock(shad, func() error {
		if _, err := os.Stat(shadDigest); err != nil {
			if err := validateCachedDigest(shadDigest, o.expectedDigest); err != nil {
				return err
			}
		} else {
			if err := validateLocalFileDigest(shadData, o.expectedDigest); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	res := &Result{
		Status:          StatusUsedCache,
		CachePath:       shadData,
		LastModified:    readTime(shadTime),
		ContentType:     readFile(shadType),
		ValidatedDigest: o.expectedDigest != "",
	}
	return res, nil
}

// cacheDirectoryPath returns the cache subdirectory path.
//   - "url" file contains the url
//   - "data" file contains the data
//   - "time" file contains the time (Last-Modified header)
//   - "type" file contains the type (Content-Type header)
func cacheDirectoryPath(cacheDir, remote string) string {
	return filepath.Join(cacheDir, "download", "by-url-sha256", CacheKey(remote))
}

// cacheDigestPath returns the cache digest file path.
//   - "<ALGO>.digest" contains the digest
func cacheDigestPath(shad string, expectedDigest digest.Digest) (string, error) {
	shadDigest := ""
	if expectedDigest != "" {
		algo := expectedDigest.Algorithm().String()
		if strings.Contains(algo, "/") || strings.Contains(algo, "\\") {
			return "", fmt.Errorf("invalid digest algorithm %q", algo)
		}
		shadDigest = filepath.Join(shad, algo+".digest")
	}
	return shadDigest, nil
}

func IsLocal(s string) bool {
	return !strings.Contains(s, "://") || strings.HasPrefix(s, "file://")
}

// canonicalLocalPath canonicalizes the local path string.
//   - Make sure the file has no scheme, or the `file://` scheme
//   - If it has the `file://` scheme, strip the scheme and make sure the filename is absolute
//   - Expand a leading `~`, or convert relative to absolute name
func canonicalLocalPath(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("got empty path")
	}
	if !IsLocal(s) {
		return "", fmt.Errorf("got non-local path: %q", s)
	}
	if strings.HasPrefix(s, "file://") {
		res := strings.TrimPrefix(s, "file://")
		if !filepath.IsAbs(res) {
			return "", fmt.Errorf("got non-absolute path %q", res)
		}
		return res, nil
	}
	return localpathutil.Expand(s)
}

func copyLocal(ctx context.Context, dst, src, ext string, decompress bool, description string, expectedDigest digest.Digest) error {
	srcPath, err := canonicalLocalPath(src)
	if err != nil {
		return err
	}

	if expectedDigest != "" {
		logrus.Debugf("verifying digest of local file %q (%s)", srcPath, expectedDigest)
	}
	if err := validateLocalFileDigest(srcPath, expectedDigest); err != nil {
		return err
	}

	if dst == "" {
		// empty dst means caching-only mode
		return nil
	}
	dstPath, err := canonicalLocalPath(dst)
	if err != nil {
		return err
	}
	if decompress {
		command := decompressor(ext)
		if command != "" {
			return decompressLocal(ctx, command, dstPath, srcPath, ext, description)
		}
		commandByMagic := decompressorByMagic(srcPath)
		if commandByMagic != "" {
			return decompressLocal(ctx, commandByMagic, dstPath, srcPath, ext, description)
		}
	}
	// TODO: progress bar for copy
	return fs.CopyFile(dstPath, srcPath)
}

func decompressor(ext string) string {
	switch ext {
	case ".gz":
		return "gzip"
	case ".bz2":
		return "bzip2"
	case ".xz":
		return "xz"
	case ".zst":
		return "zstd"
	default:
		return ""
	}
}

func decompressorByMagic(file string) string {
	f, err := os.Open(file)
	if err != nil {
		return ""
	}
	defer f.Close()
	header := make([]byte, 6)
	if _, err := f.Read(header); err != nil {
		return ""
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	if bytes.HasPrefix(header, []byte{0x1f, 0x8b}) {
		return "gzip"
	}
	if bytes.HasPrefix(header, []byte{0x42, 0x5a}) {
		return "bzip2"
	}
	if bytes.HasPrefix(header, []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}) {
		return "xz"
	}
	if bytes.HasPrefix(header, []byte{0x28, 0xb5, 0x2f, 0xfd}) {
		return "zstd"
	}
	return ""
}

func decompressLocal(ctx context.Context, decompressCmd, dst, src, ext, description string) error {
	logrus.Infof("decompressing %s with %v", ext, decompressCmd)

	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	bar, err := progressbar.New(st.Size())
	if err != nil {
		return err
	}
	if HideProgress {
		hideBar(bar)
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	buf := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, decompressCmd, "-d") // -d --decompress
	cmd.Stdin = bar.NewProxyReader(in)
	cmd.Stdout = out
	cmd.Stderr = buf
	if !HideProgress {
		if description == "" {
			description = filepath.Base(src)
		}
		logrus.Infof("Decompressing %s\n", description)
	}
	bar.Start()
	err = cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = buf.Bytes()
		}
	}
	bar.Finish()
	return err
}

func validateCachedDigest(shadDigest string, expectedDigest digest.Digest) error {
	if expectedDigest == "" {
		return nil
	}
	shadDigestB, err := os.ReadFile(shadDigest)
	if err != nil {
		return err
	}
	shadDigestS := strings.TrimSpace(string(shadDigestB))
	if shadDigestS != expectedDigest.String() {
		return fmt.Errorf("expected digest %q, got %q", expectedDigest, shadDigestS)
	}
	return nil
}

func validateLocalFileDigest(localPath string, expectedDigest digest.Digest) error {
	if localPath == "" {
		return fmt.Errorf("validateLocalFileDigest: got empty localPath")
	}
	if expectedDigest == "" {
		return nil
	}
	algo := expectedDigest.Algorithm()
	if !algo.Available() {
		return fmt.Errorf("expected digest algorithm %q is not available", algo)
	}
	r, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer r.Close()
	actualDigest, err := algo.FromReader(r)
	if err != nil {
		return err
	}
	if actualDigest != expectedDigest {
		return fmt.Errorf("expected digest %q, got %q", expectedDigest, actualDigest)
	}
	return nil
}

// mathLastModified takes params:
//   - ctx: context for calling httpclientutil.Head
//   - lastModifiedPath: path of the cached last-modified time file
//   - url: URL to fetch the last-modified time
//
// returns:
//   - matched: whether the last-modified time matches
//   - lmCached: last-modified time string from the lastModifiedPath
//   - lmRemote: last-modified time string from the URL
//   - err: error if fetching the last-modified time from the URL fails
func matchLastModified(ctx context.Context, lastModifiedPath, url string) (matched bool, lmCached, lmRemote string, err error) {
	lmCached = readFile(lastModifiedPath)
	if lmCached == "" {
		return false, "<not cached>", "<not checked>", nil
	}
	resp, err := httpclientutil.Head(ctx, http.DefaultClient, url)
	if err != nil {
		return false, lmCached, "<failed to fetch remote>", err
	}
	defer resp.Body.Close()
	lmRemote = resp.Header.Get("Last-Modified")
	if lmRemote == "" {
		return false, lmCached, "<missing Last-Modified header>", nil
	}
	lmCachedTime, errParsingCachedTime := time.Parse(http.TimeFormat, lmCached)
	lmRemoteTime, errParsingRemoteTime := time.Parse(http.TimeFormat, lmRemote)
	if errParsingCachedTime != nil && errParsingRemoteTime != nil {
		// both time strings are failed to parse, so compare them as strings
		return lmCached == lmRemote, lmCached, lmRemote, nil
	} else if errParsingCachedTime == nil && errParsingRemoteTime == nil {
		// both time strings are successfully parsed, so compare them as times
		return lmRemoteTime.Equal(lmCachedTime), lmCached, lmRemote, nil
	}
	// ignore parsing errors for either time string and assume they are different
	return false, lmCached, lmRemote, nil
}

func downloadHTTP(ctx context.Context, localPath, lastModified, contentType, url, description string, expectedDigest digest.Digest) error {
	if localPath == "" {
		return fmt.Errorf("downloadHTTP: got empty localPath")
	}
	logrus.Debugf("downloading %q into %q", url, localPath)

	resp, err := httpclientutil.Get(ctx, http.DefaultClient, url)
	if err != nil {
		return err
	}
	if lastModified != "" {
		lm := resp.Header.Get("Last-Modified")
		if err := os.WriteFile(lastModified, []byte(lm), 0o644); err != nil {
			return err
		}
	}
	if contentType != "" {
		ct := resp.Header.Get("Content-Type")
		if err := os.WriteFile(contentType, []byte(ct), 0o644); err != nil {
			return err
		}
	}
	defer resp.Body.Close()
	bar, err := progressbar.New(resp.ContentLength)
	if err != nil {
		return err
	}
	if HideProgress {
		hideBar(bar)
	}

	localPathTmp := perProcessTempfile(localPath)
	fileWriter, err := os.Create(localPathTmp)
	if err != nil {
		return err
	}
	defer fileWriter.Close()
	defer os.RemoveAll(localPathTmp)

	writers := []io.Writer{fileWriter}
	var digester digest.Digester
	if expectedDigest != "" {
		algo := expectedDigest.Algorithm()
		if !algo.Available() {
			return fmt.Errorf("unsupported digest algorithm %q", algo)
		}
		digester = algo.Digester()
		hasher := digester.Hash()
		writers = append(writers, hasher)
	}
	multiWriter := io.MultiWriter(writers...)

	if !HideProgress {
		if description == "" {
			description = url
		}
		// stderr corresponds to the progress bar output
		fmt.Fprintf(os.Stderr, "Downloading %s\n", description)
	}
	bar.Start()
	if _, err := io.Copy(multiWriter, bar.NewProxyReader(resp.Body)); err != nil {
		return err
	}
	bar.Finish()

	if digester != nil {
		actualDigest := digester.Digest()
		if actualDigest != expectedDigest {
			return fmt.Errorf("expected digest %q, got %q", expectedDigest, actualDigest)
		}
	}

	if err := fileWriter.Sync(); err != nil {
		return err
	}
	if err := fileWriter.Close(); err != nil {
		return err
	}

	return os.Rename(localPathTmp, localPath)
}

var tempfileCount atomic.Uint64

// To allow parallel download we use a per-process unique suffix for temporary
// files. Renaming the temporary file to the final file is safe without
// synchronization on posix.
// To make it easy to test we also include a counter ensuring that each
// temporary file is unique in the same process.
// https://github.com/lima-vm/lima/issues/2722
func perProcessTempfile(path string) string {
	return fmt.Sprintf("%s.tmp.%d.%d", path, os.Getpid(), tempfileCount.Add(1))
}

// CacheEntries returns a map of cache entries.
// The key is the SHA256 of the URL.
// The value is the path to the cache entry.
func CacheEntries(opts ...Opt) (map[string]string, error) {
	entries := make(map[string]string)
	var o options
	if err := o.apply(opts); err != nil {
		return nil, err
	}
	if o.cacheDir == "" {
		return entries, nil
	}
	downloadDir := filepath.Join(o.cacheDir, "download", "by-url-sha256")
	_, err := os.Stat(downloadDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return entries, nil
		}
		return nil, err
	}
	cacheEntries, err := os.ReadDir(downloadDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range cacheEntries {
		entries[entry.Name()] = filepath.Join(downloadDir, entry.Name())
	}
	return entries, nil
}

// CacheKey returns the key for a cache entry of the remote URL.
func CacheKey(remote string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(remote)))
}

// RemoveAllCacheDir removes the cache directory.
func RemoveAllCacheDir(opts ...Opt) error {
	var o options
	if err := o.apply(opts); err != nil {
		return err
	}
	if o.cacheDir == "" {
		return nil
	}
	logrus.Infof("Pruning %q", o.cacheDir)
	return os.RemoveAll(o.cacheDir)
}
