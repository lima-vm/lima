package downloader

import (
	"bytes"
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

	"github.com/cheggaaa/pb/v3"
	"github.com/containerd/continuity/fs"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/lima/pkg/progressbar"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

// HideProgress is used only for testing
var HideProgress bool

// hideBar is used only for testing
func hideBar(bar *pb.ProgressBar) {
	bar.Set(pb.ReturnSymbol, "")
	bar.SetTemplateString("")
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
	ValidatedDigest bool
}

type options struct {
	cacheDir       string // default: empty (disables caching)
	decompress     bool   // default: false (keep compression)
	description    string // default: url
	expectedDigest digest.Digest
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
// - The digest was not specified.
// - The file already exists in the local target path.
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

// Download downloads the remote resource into the local path.
//
// Download caches the remote resource if WithCache or WithCacheDir option is specified.
// Local files are not cached.
//
// When the local path already exists, Download returns Result with StatusSkipped.
// (So, the local path cannot be set to /dev/null for "caching only" mode.)
//
// The local path can be an empty string for "caching only" mode.
func Download(local, remote string, opts ...Opt) (*Result, error) {
	var o options
	for _, f := range opts {
		if err := f(&o); err != nil {
			return nil, err
		}
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
		if err := copyLocal(localPath, remote, ext, o.decompress, o.description, o.expectedDigest); err != nil {
			return nil, err
		}
		res := &Result{
			Status:          StatusDownloaded,
			ValidatedDigest: o.expectedDigest != "",
		}
		return res, nil
	}

	if o.cacheDir == "" {
		if err := downloadHTTP(localPath, remote, o.description, o.expectedDigest); err != nil {
			return nil, err
		}
		res := &Result{
			Status:          StatusDownloaded,
			ValidatedDigest: o.expectedDigest != "",
		}
		return res, nil
	}

	shad := cacheDirectoryPath(o.cacheDir, remote)
	shadData := filepath.Join(shad, "data")
	shadDigest, err := cacheDigestPath(shad, o.expectedDigest)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(shadData); err == nil {
		logrus.Debugf("file %q is cached as %q", localPath, shadData)
		if _, err := os.Stat(shadDigest); err == nil {
			logrus.Debugf("Comparing digest %q with the cached digest file %q, not computing the actual digest of %q",
				o.expectedDigest, shadDigest, shadData)
			if err := validateCachedDigest(shadDigest, o.expectedDigest); err != nil {
				return nil, err
			}
			if err := copyLocal(localPath, shadData, ext, o.decompress, "", ""); err != nil {
				return nil, err
			}
		} else {
			if err := copyLocal(localPath, shadData, ext, o.decompress, o.description, o.expectedDigest); err != nil {
				return nil, err
			}
		}
		res := &Result{
			Status:          StatusUsedCache,
			CachePath:       shadData,
			ValidatedDigest: o.expectedDigest != "",
		}
		return res, nil
	}
	if err := os.RemoveAll(shad); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(shad, 0o700); err != nil {
		return nil, err
	}
	shadURL := filepath.Join(shad, "url")
	if err := os.WriteFile(shadURL, []byte(remote), 0o644); err != nil {
		return nil, err
	}
	if err := downloadHTTP(shadData, remote, o.description, o.expectedDigest); err != nil {
		return nil, err
	}
	// no need to pass the digest to copyLocal(), as we already verified the digest
	if err := copyLocal(localPath, shadData, ext, o.decompress, "", ""); err != nil {
		return nil, err
	}
	if shadDigest != "" && o.expectedDigest != "" {
		if err := os.WriteFile(shadDigest, []byte(o.expectedDigest.String()), 0o644); err != nil {
			return nil, err
		}
	}
	res := &Result{
		Status:          StatusDownloaded,
		CachePath:       shadData,
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
	for _, f := range opts {
		if err := f(&o); err != nil {
			return nil, err
		}
	}
	if o.cacheDir == "" {
		return nil, fmt.Errorf("caching-only mode requires the cache directory to be specified")
	}
	if IsLocal(remote) {
		return nil, fmt.Errorf("local files are not cached")
	}

	shad := cacheDirectoryPath(o.cacheDir, remote)
	shadData := filepath.Join(shad, "data")
	shadDigest, err := cacheDigestPath(shad, o.expectedDigest)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(shadData); err != nil {
		return nil, err
	}
	if _, err := os.Stat(shadDigest); err != nil {
		if err := validateCachedDigest(shadDigest, o.expectedDigest); err != nil {
			return nil, err
		}
	} else {
		if err := validateLocalFileDigest(shadData, o.expectedDigest); err != nil {
			return nil, err
		}
	}
	res := &Result{
		Status:          StatusUsedCache,
		CachePath:       shadData,
		ValidatedDigest: o.expectedDigest != "",
	}
	return res, nil
}

// cacheDirectoryPath returns the cache subdirectory path.
// - "url" file contains the url
// - "data" file contains the data
func cacheDirectoryPath(cacheDir, remote string) string {
	return filepath.Join(cacheDir, "download", "by-url-sha256", fmt.Sprintf("%x", sha256.Sum256([]byte(remote))))
}

// cacheDigestPath returns the cache digest file path.
// - "<ALGO>.digest" contains the digest
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
// - Make sure the file has no scheme, or the `file://` scheme
// - If it has the `file://` scheme, strip the scheme and make sure the filename is absolute
// - Expand a leading `~`, or convert relative to absolute name
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

func copyLocal(dst, src, ext string, decompress bool, description string, expectedDigest digest.Digest) error {
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
	if _, ok := Decompressor(ext); ok && decompress {
		return decompressLocal(dstPath, srcPath, ext, description)
	}
	// TODO: progress bar for copy
	return fs.CopyFile(dstPath, srcPath)
}

func Decompressor(ext string) ([]string, bool) {
	var program string
	switch ext {
	case ".gz":
		program = "gzip"
	case ".bz2":
		program = "bzip2"
	case ".xz":
		program = "xz"
	case ".zst":
		program = "zstd"
	default:
		return nil, false
	}
	// -d --decompress
	return []string{program, "-d"}, true
}

func decompressLocal(dst, src, ext, description string) error {
	command, found := Decompressor(ext)
	if !found {
		return fmt.Errorf("decompressLocal: unknown extension %s", ext)
	}
	logrus.Infof("decompressing %s with %v", ext, command)

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
	cmd := exec.Command(command[0], command[1:]...)
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

func downloadHTTP(localPath, url, description string, expectedDigest digest.Digest) error {
	if localPath == "" {
		return fmt.Errorf("downloadHTTP: got empty localPath")
	}
	logrus.Debugf("downloading %q into %q", url, localPath)
	localPathTmp := localPath + ".tmp"
	if err := os.RemoveAll(localPathTmp); err != nil {
		return err
	}
	fileWriter, err := os.Create(localPathTmp)
	if err != nil {
		return err
	}
	defer fileWriter.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected HTTP status %d, got %s", http.StatusOK, resp.Status)
	}
	bar, err := progressbar.New(resp.ContentLength)
	if err != nil {
		return err
	}
	if HideProgress {
		hideBar(bar)
	}

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
	if err := os.RemoveAll(localPath); err != nil {
		return err
	}
	return os.Rename(localPathTmp, localPath)
}
