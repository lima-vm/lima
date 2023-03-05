package fileutils

import (
	"fmt"

	"github.com/lima-vm/lima/pkg/downloader"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/sirupsen/logrus"
)

// DownloadFile downloads a file to the cache, optionally copying it to the destination. Returns path in cache.
func DownloadFile(dest string, f limayaml.File, description string, expectedArch limayaml.Arch) (string, error) {
	if f.Arch != expectedArch {
		return "", fmt.Errorf("unsupported arch: %q", f.Arch)
	}
	logrus.WithField("digest", f.Digest).Infof("Attempting to download %s from %q", description, f.Location)
	res, err := downloader.Download(dest, f.Location,
		downloader.WithCache(),
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
