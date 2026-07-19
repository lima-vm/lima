// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fsutil

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// WindowsSubsystemPath converts a Windows path to a Cygwin/MSYS form
// (C:\Users\jan -> /c/Users/jan) via cygpath on PATH. A caller holding a
// specific ssh toolchain should use WindowsSubsystemPathWithCygpath, which
// runs that toolchain's own cygpath and respects its fstab.
func WindowsSubsystemPath(ctx context.Context, orig string) (string, error) {
	return WindowsSubsystemPathWithCygpath(ctx, "cygpath", orig)
}

// WindowsSubsystemPathWithCygpath converts a Windows path with the given
// cygpathExe ("cygpath" for PATH, or an absolute path to bind the conversion
// to one Cygwin install). When the cygpath binary is not found it falls back
// to a native conversion of the absolute drive-letter case; a cygpath that
// runs and fails returns its error, and non-drive-letter inputs (UNC, device,
// extended-length) return an error.
func WindowsSubsystemPathWithCygpath(ctx context.Context, cygpathExe, orig string) (string, error) {
	out, err := exec.CommandContext(ctx, cygpathExe, filepath.ToSlash(orig)).CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	if !errors.Is(err, exec.ErrNotFound) && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("failed to run %#q on %#q: %#q: %w", cygpathExe, orig, strings.TrimSpace(string(out)), err)
	}

	logrus.WithError(err).Debugf("%#q not found for %#q, attempting native conversion", cygpathExe, orig)

	return windowsSubsystemPathWithoutCygpath(orig)
}

func windowsSubsystemPathWithoutCygpath(orig string) (string, error) {
	// The /c/... form this produces is the MSYS2 and Git-for-Windows
	// convention; stock Cygwin defaults to /cygdrive/c/. Only an absolute
	// drive-letter path ("C:\foo") has a well-defined form here. A
	// drive-relative path ("C:foo") would become an unrelated absolute
	// path, so reject it.
	if !filepath.IsAbs(orig) {
		return "", fmt.Errorf("cannot convert %#q to an MSYS-style path: input is not an absolute drive-letter path", orig)
	}

	// UNC path ("\\server\share\foo") is rejected here.
	if vol := filepath.VolumeName(orig); len(vol) == 2 && vol[1] == ':' {
		// orig[2:] starts with a separator for an absolute drive path
		// (C:\foo, C:/foo); strip it so the result stays canonical.
		tail := strings.TrimPrefix(filepath.ToSlash(orig[2:]), "/")
		converted := "/" + strings.ToLower(vol[:1]) + "/" + tail
		logrus.Debugf("native cygpath fallback: %#q -> %#q", orig, converted)
		return converted, nil
	}

	return "", fmt.Errorf("cannot convert %#q to an MSYS-style path: input is not an absolute drive-letter path", orig)
}

// WindowsSubsystemPathForLinux converts a Windows path to the WSL form of
// the given distro (e.g. C:\Users\jan -> /mnt/c/Users/jan) via wsl.exe.
func WindowsSubsystemPathForLinux(ctx context.Context, orig, distro string) (string, error) {
	out, err := exec.CommandContext(ctx, "wsl", "-d", distro, "--exec", "wslpath", filepath.ToSlash(orig)).CombinedOutput()
	if err != nil {
		logrus.WithError(err).Errorf("failed to convert path to mingw, maybe wsl command is not operational?")
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
