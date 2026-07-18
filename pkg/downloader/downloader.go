// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package downloader

import (
	"bytes"
	"compress/gzip"
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
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/containerd/continuity/fs"
	"github.com/lima-vm/go-qcow2reader/image/raw"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/httpclientutil"
	"github.com/lima-vm/lima/v2/pkg/imgutil/nativeimgutil"
	"github.com/lima-vm/lima/v2/pkg/imgutil/proxyimgutil"
	"github.com/lima-vm/lima/v2/pkg/iso9660util"
	"github.com/lima-vm/lima/v2/pkg/localpathutil"
	"github.com/lima-vm/lima/v2/pkg/lockutil"
	"github.com/lima-vm/lima/v2/pkg/progressbar"
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
	cacheDir              string // default: empty (disables caching)
	decompress            bool   // default: false (keep compression)
	description           string // default: url
	expectedDigest        digest.Digest
	supportedImageFormats []string
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

func WithImageFormats(supportedImageFormats []string) Opt {
	return func(o *options) error {
		o.supportedImageFormats = supportedImageFormats
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
			return nil, errors.New("caching-only mode requires the cache directory to be specified")
		}
	} else {
		var err error
		localPath, err = canonicalLocalPath(local)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(localPath); err == nil {
			logrus.Debugf("file %#q already exists, skipping downloading from %#q (and skipping digest validation)", localPath, remote)
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
		if err := downloadHTTP(ctx, httpTarget{localPath: localPath}, remote, o.description, o.expectedDigest); err != nil {
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
	if _, err := os.Stat(shadDigest); err == nil {
		if err := validateCachedDigest(shadDigest, o.expectedDigest); err != nil {
			return nil, err
		}
	} else if match, lmCached, lmRemote, err := matchLastModified(ctx, shadTime, remote); err != nil {
		logrus.WithError(err).Info("Failed to retrieve last-modified for cached digest-less image; using cached image.")
	} else if !match {
		logrus.Infof("Re-downloading digest-less image: last-modified mismatch (cached: %#q, remote: %#q)", lmCached, lmRemote)
		return nil, nil
	}

	// Some drivers (e.g. vz) can only boot raw images. When the driver tells us
	// which formats it supports and the cached image is not one of them, we save
	// a converted raw copy next to the original at <shad>/imgconv/raw. See
	// website/content/en/docs/dev/internals.md for the layout.
	if len(o.supportedImageFormats) > 0 {
		isISO, err := iso9660util.IsISO9660(shadData)
		if err != nil {
			logrus.WithError(err).Debugf("Skipping cache image conversion for %q (unable to check ISO9660)", shadData)
		} else if !isISO {
			imageFormat, err := nativeimgutil.DetectFormat(shadData)
			if err != nil {
				logrus.WithError(err).Debugf("Skipping cache image conversion for %q (unable to detect format)", shadData)
			} else if !slices.Contains(o.supportedImageFormats, imageFormat) {
				rawImgConvPath := filepath.Join(shad, "imgconv", "raw")
				rawImgConvDigestPath := filepath.Join(shad, "imgconv", "raw.digest")

				needConvert := false
				if rawStat, err := os.Stat(rawImgConvPath); err != nil {
					if errors.Is(err, os.ErrNotExist) {
						needConvert = true
					} else {
						return nil, err
					}
				} else {
					origStat, err := os.Stat(shadData)
					if err != nil {
						return nil, err
					}
					if origStat.ModTime().After(rawStat.ModTime()) {
						needConvert = true
					}
				}

				if needConvert {
					logrus.Infof("Converted raw image is missing or stale; (re)converting now.")
					converted, rawDigest, err := ensureRawInCache(ctx, shadData, imageFormat, o.expectedDigest)
					if err != nil {
						return nil, err
					}
					shadData = converted
					if o.expectedDigest != "" {
						o.expectedDigest = rawDigest
						shadDigest = rawImgConvDigestPath
					}
				} else {
					shadData = rawImgConvPath
					if o.expectedDigest != "" {
						if currentDigestData, err := os.ReadFile(rawImgConvDigestPath); err == nil {
							currentDigest := strings.TrimSpace(string(currentDigestData))
							if d, err := digest.Parse(currentDigest); err == nil {
								o.expectedDigest = d
								shadDigest = rawImgConvDigestPath
							} else {
								return nil, fmt.Errorf("invalid digest in raw digest file %q: %w", rawImgConvDigestPath, err)
							}
						} else {
							return nil, fmt.Errorf("failed to read raw digest file %q: %w", rawImgConvDigestPath, err)
						}
					}
				}
			}
		}
	}

	ext := path.Ext(remote)
	logrus.Debugf("file %#q is cached as %#q", localPath, shadData)
	if _, err := os.Stat(shadDigest); err == nil {
		logrus.Debugf("Comparing digest %#q with the cached digest file %#q, not computing the actual digest of %#q",
			o.expectedDigest, shadDigest, shadData)
		if err := validateCachedDigest(shadDigest, o.expectedDigest); err != nil {
			return nil, err
		}
		if err := copyLocal(ctx, localPath, shadData, ext, o.decompress, "", ""); err != nil {
			return nil, err
		}
	} else {
		if err := copyLocal(ctx, localPath, shadData, ext, o.decompress, o.description, o.expectedDigest); err != nil {
			return nil, err
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
	target := httpTarget{
		localPath: shadData,
		timePath:  shadTime,
		typePath:  shadType,
		etagPath:  filepath.Join(shad, "etag"),
		resume:    true,
	}
	if err := downloadHTTP(ctx, target, remote, o.description, o.expectedDigest); err != nil {
		return nil, err
	}
	if shadDigest != "" && o.expectedDigest != "" {
		if err := os.WriteFile(shadDigest, []byte(o.expectedDigest.String()), 0o644); err != nil {
			return nil, err
		}
	}

	// If the driver cannot use the format we just downloaded, save a converted
	// raw copy at <shad>/imgconv/raw, keeping the original <shad>/data as-is.
	//
	// Gated on WithImageFormats() (the caller's signal that this is a VM disk
	// image) and on the file not being an ISO 9660 image.
	if len(o.supportedImageFormats) > 0 {
		isISO, err := iso9660util.IsISO9660(shadData)
		if err != nil {
			logrus.WithError(err).Debugf("Skipping cache image conversion for %q (unable to check ISO9660)", shadData)
		} else if !isISO {
			format, err := nativeimgutil.DetectFormat(shadData)
			if err != nil {
				logrus.WithError(err).Debugf("Skipping cache image conversion for %q (unable to detect format)", shadData)
			} else if !slices.Contains(o.supportedImageFormats, format) {
				converted, rawDigest, err := ensureRawInCache(ctx, shadData, format, o.expectedDigest)
				if err != nil {
					return nil, fmt.Errorf("failed to convert image to raw: %w", err)
				} else if converted != "" {
					shadData = converted
					if o.expectedDigest != "" {
						o.expectedDigest = rawDigest
					}
				}
			}
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

// ensureRawInCache converts any image to raw and places it in the cache(imgconv/raw). It also creates a
// digest file for the raw image(imgconv/raw.digest). Returns the converted image path, the raw digest, and any error.
func ensureRawInCache(ctx context.Context, imagePath, format string, originalDigest digest.Digest) (string, digest.Digest, error) {
	imgConvPath := filepath.Join(filepath.Dir(imagePath), "imgconv")
	if err := os.MkdirAll(imgConvPath, 0o700); err != nil {
		return "", "", err
	}
	rawImgConvPath := filepath.Join(imgConvPath, "raw")

	logrus.Infof("Converting %s image to raw sparse format in cache: %q", format, rawImgConvPath)
	rawPathTmp := filepath.Join(imgConvPath, "raw.tmp")
	defer os.Remove(rawPathTmp)
	diskUtil := proxyimgutil.NewDiskUtil(ctx)
	if err := diskUtil.Convert(ctx, raw.Type, imagePath, rawPathTmp, nil, false); err != nil {
		return "", "", fmt.Errorf("failed to convert %q to raw: %w", imagePath, err)
	}

	// Ensure the image is sparse to save cache space.
	rawTmpF, err := os.OpenFile(rawPathTmp, os.O_RDWR, 0o644)
	if err != nil {
		return "", "", fmt.Errorf("failed to open raw tmp file %q: %w", rawPathTmp, err)
	}
	fi, err := rawTmpF.Stat()
	if err != nil {
		_ = rawTmpF.Close()
		return "", "", fmt.Errorf("failed to stat raw tmp file %q: %w", rawPathTmp, err)
	}
	if err := diskUtil.MakeSparse(ctx, rawTmpF, fi.Size()); err != nil {
		logrus.WithError(err).Warnf("Failed to make %q sparse (non-fatal)", rawPathTmp)
	}
	if err := rawTmpF.Close(); err != nil {
		return "", "", fmt.Errorf("failed to close raw tmp file %q: %w", rawPathTmp, err)
	}

	if err := os.Remove(rawImgConvPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("failed to remove stale raw image %q: %w", rawImgConvPath, err)
	}
	if err := os.Rename(rawPathTmp, rawImgConvPath); err != nil {
		return "", "", fmt.Errorf("failed to replace original with raw image: %w", err)
	}

	algo := digest.Canonical
	if originalDigest != "" {
		algo = originalDigest.Algorithm()
	}
	rawDigest, err := calculateFileDigest(rawImgConvPath, algo)
	if err != nil {
		return "", "", fmt.Errorf("failed to calculate digest of raw image: %w", err)
	}

	rawDigestPath := filepath.Join(imgConvPath, "raw.digest")
	rawDigestPathTmp := rawDigestPath + ".tmp"
	defer os.Remove(rawDigestPathTmp)
	if err := os.WriteFile(rawDigestPathTmp, []byte(rawDigest.String()), 0o644); err != nil {
		return "", "", fmt.Errorf("failed to write raw digest file: %w", err)
	}
	if err := os.Remove(rawDigestPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("failed to remove stale raw digest file %q: %w", rawDigestPath, err)
	}
	if err := os.Rename(rawDigestPathTmp, rawDigestPath); err != nil {
		return "", "", fmt.Errorf("failed to rename raw digest file: %w", err)
	}

	return rawImgConvPath, rawDigest, nil
}

func calculateFileDigest(path string, algo digest.Algorithm) (digest.Digest, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	return algo.FromReader(f)
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
		return nil, errors.New("caching-only mode requires the cache directory to be specified")
	}
	if IsLocal(remote) {
		return nil, errors.New("local files are not cached")
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
			return "", fmt.Errorf("invalid digest algorithm %#q", algo)
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
		return "", errors.New("got empty path")
	}
	if !IsLocal(s) {
		return "", fmt.Errorf("got non-local path: %#q", s)
	}
	if res, ok := strings.CutPrefix(s, "file://"); ok {
		if !filepath.IsAbs(res) {
			return "", fmt.Errorf("got non-absolute path %#q", res)
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
		logrus.Debugf("verifying digest of local file %#q (%s)", srcPath, expectedDigest)
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
	if !HideProgress {
		if description == "" {
			description = filepath.Base(src)
		}
		logrus.Infof("Decompressing %s", description)
	}
	bar.Start()
	defer bar.Finish()
	reader := bar.NewProxyReader(in)

	// Prefer the external decompressor; fall back to a built-in pure-Go
	// gzip decoder only when the binary is missing, so a plain Windows
	// host with no gzip can still unpack a .tar.gz image. xz, bzip2, and
	// zstd stay external-only.
	if _, lookErr := exec.LookPath(decompressCmd); lookErr != nil {
		var r io.Reader
		switch decompressCmd {
		case "gzip":
			gz, err := gzip.NewReader(reader)
			if err != nil {
				return err
			}
			defer gz.Close()
			r = gz
		default:
			return fmt.Errorf("decompressor %q not found and no built-in fallback: %w", decompressCmd, lookErr)
		}
		logrus.Infof("decompressing %s with built-in %s (external %q not found)", ext, decompressCmd, decompressCmd)
		_, err = io.Copy(out, &ctxReader{ctx: ctx, r: r})
		return err
	}

	logrus.Infof("decompressing %s with external %q", ext, decompressCmd)
	buf := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, decompressCmd, "-d")
	cmd.Stdin = reader
	cmd.Stdout = out
	cmd.Stderr = buf
	err = cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitErr.Stderr = buf.Bytes()
		}
	}
	return err
}

// ctxReader makes an uninterruptible in-process decode cancellable: once ctx
// is cancelled each Read returns its error, unwinding the io.Copy that drives
// the decoder.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
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
		return fmt.Errorf("expected digest %#q, got %#q", expectedDigest, shadDigestS)
	}
	return nil
}

func validateLocalFileDigest(localPath string, expectedDigest digest.Digest) error {
	if localPath == "" {
		return errors.New("validateLocalFileDigest: got empty localPath")
	}
	if expectedDigest == "" {
		return nil
	}
	algo := expectedDigest.Algorithm()
	if !algo.Available() {
		return fmt.Errorf("expected digest algorithm %#q is not available", algo)
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
		return fmt.Errorf("expected digest %#q, got %#q", expectedDigest, actualDigest)
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

// httpTarget describes where downloadHTTP writes the downloaded data and its
// metadata. The metadata paths are optional (empty disables writing them).
type httpTarget struct {
	localPath string // final destination (e.g. shadData, or the user's file)
	timePath  string // "" or shadTime (Last-Modified)
	typePath  string // "" or shadType (Content-Type)
	etagPath  string // "" or shadETag (ETag)
	resume    bool   // use a stable .part file and attempt an HTTP Range resume
}

// ifRangeValidator returns the validator (ETag preferred, else Last-Modified)
// cached on disk. It is sent as If-Range so the server only serves a partial
// response when the resource is unchanged.
func ifRangeValidator(etagPath, timePath string) string {
	if etag := readFile(etagPath); etag != "" {
		return etag
	}
	return readFile(timePath)
}

// resumeOffset returns the number of bytes of a previous partial download that
// can be resumed. It is 0 unless the target is resumable, a non-empty partial
// file exists, and a digest is configured to verify the final result.
func resumeOffset(t httpTarget, partial string, expectedDigest digest.Digest, url string) int64 {
	if !t.resume {
		return 0
	}
	fi, err := os.Stat(partial)
	if err != nil || fi.Size() == 0 {
		return 0
	}
	if expectedDigest == "" {
		// Without a digest the integrity of the existing partial cannot be
		// verified, so restart from scratch instead of trusting possibly corrupt
		// bytes.
		logrus.Warnf("No digest configured for %#q; cannot verify the existing partial download, restarting from scratch", url)
		return 0
	}
	return fi.Size()
}

// newDownloadRequest builds the GET request, adding Range/If-Range headers when
// resuming from a non-zero offset.
func newDownloadRequest(ctx context.Context, url string, offset int64, t httpTarget) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		if ir := ifRangeValidator(t.etagPath, t.timePath); ir != "" {
			req.Header.Set("If-Range", ir)
		}
	}
	return req, nil
}

// writeResponseMetadata persists the Last-Modified, Content-Type and ETag
// response headers to the configured cache metadata files.
func writeResponseMetadata(t httpTarget, resp *http.Response) error {
	write := func(path, value string) error {
		if path == "" {
			return nil
		}
		return os.WriteFile(path, []byte(value), 0o644)
	}
	if err := write(t.timePath, resp.Header.Get("Last-Modified")); err != nil {
		return err
	}
	if err := write(t.typePath, resp.Header.Get("Content-Type")); err != nil {
		return err
	}
	return write(t.etagPath, resp.Header.Get("ETag"))
}

// openPartialFile opens the partial file for writing, appending when resuming or
// truncating for a fresh download.
func openPartialFile(partial string, resuming bool) (*os.File, error) {
	flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if resuming {
		flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	return os.OpenFile(partial, flag, 0o644)
}

// flushPartial flushes and closes the partial file. The explicit close surfaces
// any deferred write error and releases the handle, which is required before the
// file can be removed (on digest mismatch) or renamed on Windows, where an open
// file cannot be deleted or renamed.
func flushPartial(fileWriter *os.File) error {
	if err := fileWriter.Sync(); err != nil {
		return err
	}
	return fileWriter.Close()
}

// resumeDigester creates a digester for expectedDigest, seeding it with the
// already-downloaded bytes of the partial file when resuming (offset > 0). It
// returns a nil digester when no digest is expected.
func resumeDigester(expectedDigest digest.Digest, partial string, offset int64) (digest.Digester, error) {
	if expectedDigest == "" {
		return nil, nil
	}
	algo := expectedDigest.Algorithm()
	if !algo.Available() {
		return nil, fmt.Errorf("unsupported digest algorithm %#q", algo)
	}
	digester := algo.Digester()
	if offset > 0 {
		existing, err := os.Open(partial)
		if err != nil {
			return nil, err
		}
		defer existing.Close()
		if _, err := io.CopyN(digester.Hash(), existing, offset); err != nil {
			return nil, err
		}
	}
	return digester, nil
}

// verifyDownloadedDigest checks the digester result against expectedDigest. On
// mismatch it discards a resumable partial so a corrupt resume does not poison
// future retries. It is a no-op when no digest was expected (digester == nil).
func verifyDownloadedDigest(digester digest.Digester, expectedDigest digest.Digest, partial string, resume bool) error {
	if digester == nil {
		return nil
	}
	actualDigest := digester.Digest()
	if actualDigest == expectedDigest {
		return nil
	}
	if resume {
		_ = os.Remove(partial)
	}
	return fmt.Errorf("expected digest %#q, got %#q", expectedDigest, actualDigest)
}

// startProgressBar creates and starts a progress bar for the download. total
// always reflects the full size (existing offset + remaining) even when resuming,
// and the bar starts pre-filled to offset. When HideProgress is set the bar is
// silent and no description is printed.
func startProgressBar(resp *http.Response, offset int64, resuming bool, description, url string) (*progressbar.ProgressBar, error) {
	total := resp.ContentLength
	if resuming && total >= 0 {
		total += offset
	}
	bar, err := progressbar.New(total)
	if err != nil {
		return nil, err
	}
	if HideProgress {
		hideBar(bar)
	} else {
		if description == "" {
			description = url
		}
		// stderr corresponds to the progress bar output
		fmt.Fprintf(os.Stderr, "Downloading %s\n", description)
	}
	bar.Start()
	if offset > 0 {
		bar.SetCurrent(offset)
	}
	return bar, nil
}

// finalizeOrRestartPartial handles a 416 response. A 416 usually means the partial
// is already the complete file (interrupted after the body finished but before the
// final rename): if a digest is configured and the partial validates against it,
// it is finalized in place without re-downloading; otherwise the partial is
// discarded and the download restarts from scratch.
func finalizeOrRestartPartial(ctx context.Context, t httpTarget, partial string, offset int64, url, description string, expectedDigest digest.Digest) error {
	if offset > 0 && expectedDigest != "" && validateLocalFileDigest(partial, expectedDigest) == nil {
		logrus.Debugf("partial download of %#q is already complete; finalizing without re-downloading", url)
		return os.Rename(partial, t.localPath)
	}
	logrus.Debugf("discarding partial %#q and re-downloading %#q", partial, url)
	if err := os.Remove(partial); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return downloadHTTP(ctx, t, url, description, expectedDigest)
}

func downloadHTTP(ctx context.Context, t httpTarget, url, description string, expectedDigest digest.Digest) error {
	if t.localPath == "" {
		return errors.New("downloadHTTP: got empty localPath")
	}
	logrus.Debugf("downloading %#q into %#q", url, t.localPath)

	// The resumable (cache) path uses a stable partial file so that an interrupted
	// download can be continued on the next attempt. The uncached path keeps a
	// per-process temp file that must not linger.
	partial := perProcessTempfile(t.localPath)
	if t.resume {
		partial = partialFilePath(t.localPath)
		logrus.Debugf("resumable download for %#q, using partial file %#q", url, partial)
	}
	offset := resumeOffset(t, partial, expectedDigest, url)

	req, err := newDownloadRequest(ctx, url, offset, t)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	logrus.Debugf("got HTTP status %q for %#q", resp.Status, url)

	// Decide whether the server is resuming (206) or sending the whole file.
	var resuming bool
	switch {
	case resp.StatusCode == http.StatusPartialContent:
		logrus.Debugf("server returned 206 Partial Content; resuming %#q from offset %d", url, offset)
		resuming = true
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		resp.Body.Close()
		return finalizeOrRestartPartial(ctx, t, partial, offset, url, description, expectedDigest)
	case resp.StatusCode/100 == 2:
		// 200 OK, or any other 2xx: full body. The server ignored Range, or
		// If-Range signalled that the resource changed. Reset offset so the
		// digester and progress bar treat this as a fresh download.
		logrus.Debugf("server returned the full body for %#q (no Range support or the resource changed)", url)
		offset = 0
	default:
		return httpclientutil.Successful(resp)
	}

	if err := writeResponseMetadata(t, resp); err != nil {
		return err
	}

	fileWriter, err := openPartialFile(partial, resuming)
	if err != nil {
		return err
	}
	defer fileWriter.Close()
	if !t.resume {
		defer os.RemoveAll(partial)
	}

	digester, err := resumeDigester(expectedDigest, partial, offset)
	if err != nil {
		return err
	}
	writers := []io.Writer{fileWriter}
	if digester != nil {
		writers = append(writers, digester.Hash())
	}

	bar, err := startProgressBar(resp, offset, resuming, description, url)
	if err != nil {
		return err
	}
	if _, err := io.Copy(io.MultiWriter(writers...), bar.NewProxyReader(resp.Body)); err != nil {
		return err
	}
	bar.Finish()

	// Flush and close before verifying and renaming: on Windows an open file
	// cannot be removed (on digest mismatch) or renamed.
	if err := flushPartial(fileWriter); err != nil {
		return err
	}
	if err := verifyDownloadedDigest(digester, expectedDigest, partial, t.resume); err != nil {
		return err
	}
	return os.Rename(partial, t.localPath)
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

// partialFilePath returns the stable partial-download path for a cache entry.
// It is safe to reuse across processes because the cache download runs under an
// exclusive directory lock (see Download), so only one process writes it at a
// time. A single stable name also enables HTTP Range resume and bounds leftover
// partial files to at most one per URL.
func partialFilePath(path string) string {
	return path + ".part"
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
	logrus.Infof("Pruning %#q", o.cacheDir)
	return os.RemoveAll(o.cacheDir)
}
