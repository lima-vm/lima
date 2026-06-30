// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cacheutil

import (
	"context"
	"fmt"
	"path"

	"github.com/lima-vm/lima/v2/pkg/downloader"
	"github.com/lima-vm/lima/v2/pkg/fileutils"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func NerdctlArchive(y *limatype.LimaYAML) string {
	if *y.Containerd.System || *y.Containerd.User {
		for _, f := range y.Containerd.Archives {
			if f.Arch == *y.Arch {
				return path.Base(f.Location)
			}
		}
	}
	return ""
}

func EnsureNerdctlArchiveCache(ctx context.Context, y *limatype.LimaYAML, created bool) (string, error) {
	if !*y.Containerd.System && !*y.Containerd.User {
		return "", nil
	}

	errs := make([]error, len(y.Containerd.Archives))

	for i, f := range y.Containerd.Archives {
		if created && f.Arch == *y.Arch && !downloader.IsLocal(f.Location) {
			path, err := fileutils.CachedFile(f)
			if err == nil {
				return path, nil
			}
		}
		path, err := fileutils.DownloadFile(ctx, "", f, false, "the nerdctl archive", *y.Arch)
		if err != nil {
			errs[i] = err
			continue
		}
		if path == "" {
			if downloader.IsLocal(f.Location) {
				return f.Location, nil
			}
			return "", fmt.Errorf("cache did not contain %#q", f.Location)
		}
		return path, nil
	}

	return "", fileutils.Errors(errs)
}
