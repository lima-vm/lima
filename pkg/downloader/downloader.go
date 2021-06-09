package downloader

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AkihiroSuda/lima/pkg/localpathutil"
	"github.com/containerd/continuity/fs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Status = string

const (
	StatusUnknown    Status = ""
	StatusDownloaded Status = "downloaded"
	StatusSkipped    Status = "skipped"
	StatusUsedCache  Status = "used-cache"
)

type Result struct {
	Status    Status
	CachePath string // "/Users/foo/Library/Caches/lima/download/by-url-sha256/<SHA256_OF_URL>/data"
}

type options struct {
	cacheDir string // default: empty (disables caching)
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

func Download(local, remote string, opts ...Opt) (*Result, error) {
	var o options
	for _, f := range opts {
		if err := f(&o); err != nil {
			return nil, err
		}
	}
	localPath, err := localPath(local)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(localPath); err == nil {
		logrus.Debugf("file %q already exists, skipping downloading from %q", localPath, remote)
		res := &Result{
			Status: StatusSkipped,
		}
		return res, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	localPathDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localPathDir, 0755); err != nil {
		return nil, err
	}

	if isLocal(remote) {
		if err := copyLocal(localPath, remote); err != nil {
			return nil, err
		}
		res := &Result{
			Status: StatusDownloaded,
		}
		return res, nil
	}

	if o.cacheDir == "" {
		if err := downloadHTTP(localPath, remote); err != nil {
			return nil, err
		}
		res := &Result{
			Status: StatusDownloaded,
		}
		return res, nil
	}

	shad := filepath.Join(o.cacheDir, "download", "by-url-sha256", fmt.Sprintf("%x", sha256.Sum256([]byte(remote))))
	shadData := filepath.Join(shad, "data")
	if _, err := os.Stat(shadData); err == nil {
		logrus.Debugf("file %q is cached as %q", localPath, shadData)
		if err := copyLocal(localPath, shadData); err != nil {
			return nil, err
		}
		res := &Result{
			Status:    StatusUsedCache,
			CachePath: shadData,
		}
		return res, nil
	}
	if err := os.RemoveAll(shad); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(shad, 0700); err != nil {
		return nil, err
	}
	shadURL := filepath.Join(shad, "url")
	if err := os.WriteFile(shadURL, []byte(remote), 0644); err != nil {
		return nil, err
	}
	if err := downloadHTTP(shadData, remote); err != nil {
		return nil, err
	}
	if err := copyLocal(localPath, shadData); err != nil {
		return nil, err
	}

	res := &Result{
		Status:    StatusDownloaded,
		CachePath: shadData,
	}
	return res, nil
}

func isLocal(s string) bool {
	return !strings.Contains(s, "://") || strings.HasPrefix(s, "file://")
}

func localPath(s string) (string, error) {
	if !isLocal(s) {
		return "", errors.Errorf("got non-local path: %q", s)
	}
	if strings.HasPrefix(s, "file://") {
		res := strings.TrimPrefix(s, "file://")
		if !filepath.IsAbs(res) {
			return "", errors.Errorf("got non-absolute path %q", res)
		}
		return res, nil
	}
	return localpathutil.Expand(s)
}

func copyLocal(dst, src string) error {
	srcPath, err := localPath(src)
	if err != nil {
		return err
	}
	dstPath, err := localPath(dst)
	if err != nil {
		return err
	}
	return fs.CopyFile(dstPath, srcPath)
}

func downloadHTTP(localPath, url string) error {
	logrus.Debugf("downloading %q into %q", url, localPath)
	localPathTmp := localPath + ".tmp"
	if err := os.RemoveAll(localPathTmp); err != nil {
		return err
	}
	// use curl for printing progress
	cmd := exec.Command("curl", "-fSL", "-o", localPathTmp, url)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to run %v", cmd.Args)
	}
	if err := os.RemoveAll(localPath); err != nil {
		return err
	}
	if err := os.Rename(localPathTmp, localPath); err != nil {
		return err
	}
	return nil
}
