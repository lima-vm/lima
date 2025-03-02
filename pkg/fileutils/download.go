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

package fileutils

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/lima-vm/lima/pkg/downloader"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/sirupsen/logrus"
)

// ErrSkipped is returned when the downloader did not attempt to download the specified file.
var ErrSkipped = errors.New("skipped to download")

// DownloadFile downloads a file to the cache, optionally copying it to the destination. Returns path in cache.
func DownloadFile(ctx context.Context, dest string, f limayaml.File, decompress bool, description string, expectedArch limayaml.Arch) (string, error) {
	if f.Arch != expectedArch {
		return "", fmt.Errorf("%w: %q: unsupported arch: %q", ErrSkipped, f.Location, f.Arch)
	}
	fields := logrus.Fields{"location": f.Location, "arch": f.Arch, "digest": f.Digest}
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
