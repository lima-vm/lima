// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fileutils

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/downloader"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
)

// ErrSkipped is returned when the downloader did not attempt to download the specified file.
var ErrSkipped = errors.New("skipped to download")

func CopyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o660); err != nil {
		return err
	}
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}

func GetFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil)), nil
}

// DownloadFile downloads a file to the cache, optionally copying it to the destination. Returns path in cache.
func DownloadFile(ctx context.Context, dest string, f limayaml.File, decompress bool, description string, expectedArch limayaml.Arch) (_ string, reterr error) {
	if f.Arch != expectedArch {
		return "", fmt.Errorf("%w: %q: unsupported arch: %q", ErrSkipped, f.Location, f.Arch)
	}
	fields := logrus.Fields{"location": f.Location, "arch": f.Arch, "digest": f.Digest, "LocalPath": f.LocalPath}
	if f.LocalPath != "" {
		if _, err := os.Stat(f.LocalPath); err != nil {
			return "", err
		}
		logrus.WithFields(fields).Infof("Attempting to copy local file %s", description)
		if reterr != nil {
			defer os.Remove(dest)
		}
		if err := CopyFile(f.LocalPath, dest); err != nil {
			return "", fmt.Errorf("failed to copy file: %w", err)
		}
		sha256Sum, err := GetFileSHA256(dest)
		if err != nil {
			return "", fmt.Errorf("failed to getsha256: %w", err)
		}

		if sha256Sum != f.Digest.String() {
			return "", fmt.Errorf("wrong sha256 for %s", dest)
		}
		return f.LocalPath, nil
	}

	logrus.WithFields(fields).Infof("Attempting to download %s", description)
	res, err := downloader.Download(ctx, dest, f.Location,
		downloader.WithCache(),
		downloader.WithDecompress(decompress),
		downloader.WithDescription(fmt.Sprintf("%s (%s)", description, path.Base(f.Location))),
		downloader.WithExpectedDigest(f.Digest),
	)
	if err != nil {
		return "", fmt.Errorf("failed to download %q: %w", f.Location, err)
	}
	logrus.Debugf("res.ValidatedDigest=%v", res.ValidatedDigest)
	switch res.Status {
	case downloader.StatusDownloaded:
		logrus.Infof("Downloaded %s from %q", description, f.Location)
	case downloader.StatusUsedCache:
		logrus.Infof("Using cache %q", res.CachePath)
	default:
		logrus.Warnf("Unexpected result from downloader.Download(): %+v", res)
	}
	return res.CachePath, nil
}

// CachedFile checks if a file is in the cache, validating the digest if it is available. Returns path in cache.
func CachedFile(f limayaml.File) (string, error) {
	res, err := downloader.Cached(f.Location,
		downloader.WithCache(),
		downloader.WithExpectedDigest(f.Digest))
	if err != nil {
		return "", fmt.Errorf("cache did not contain %q: %w", f.Location, err)
	}
	return res.CachePath, nil
}

// Errors compose multiple into a single error.
// Errors filters out ErrSkipped.
func Errors(errs []error) error {
	var finalErr error
	for _, err := range errs {
		if errors.Is(err, ErrSkipped) {
			logrus.Debug(err)
		} else {
			finalErr = errors.Join(finalErr, err)
		}
	}
	if len(errs) > 0 && finalErr == nil {
		// errs only contains ErrSkipped
		finalErr = fmt.Errorf("%v", errs)
	}
	return finalErr
}
