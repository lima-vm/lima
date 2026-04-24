// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ioutilx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// ReadAtMaximum reads n at maximum.
func ReadAtMaximum(r io.Reader, n int64) ([]byte, error) {
	lr := &io.LimitedReader{
		R: r,
		N: n,
	}
	b, err := io.ReadAll(lr)
	if err != nil {
		if errors.Is(err, io.EOF) && lr.N <= 0 {
			err = fmt.Errorf("exceeded the limit (%d bytes): %w", n, err)
		}
	}
	return b, err
}

// FromUTF16le returns an io.Reader for UTF16le data.
// Windows uses little endian by default, use unicode.UseBOM policy to retrieve BOM from the text,
// and unicode.LittleEndian as a fallback.
func FromUTF16le(r io.Reader) io.Reader {
	o := transform.NewReader(r, unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder())
	return o
}

// FromUTF16leToString reads from Unicode 16 LE encoded data from an io.Reader and returns a string.
func FromUTF16leToString(r io.Reader) (string, error) {
	out, err := io.ReadAll(FromUTF16le(r))
	if err != nil {
		return "", err
	}

	return string(out), nil
}

// WindowsSubsystemPath converts a Windows path to a Cygwin/MSYS-style
// path (e.g. C:\Users\jan -> /c/Users/jan) using the cygpath found via
// PATH. Callers that have a specific ssh toolchain in hand should call
// WindowsSubsystemPathWithCygpath instead, so the conversion runs
// through that toolchain's own cygpath and respects its fstab.
func WindowsSubsystemPath(ctx context.Context, orig string) (string, error) {
	return WindowsSubsystemPathWithCygpath(ctx, "cygpath", orig)
}

// WindowsSubsystemPathWithCygpath converts a Windows path to a
// Cygwin/MSYS-style path using the specific cygpath executable
// cygpathExe. Pass "cygpath" to resolve via PATH; pass an absolute path
// to bind the conversion to a particular Cygwin install (the sibling of
// an ssh.exe selected via $SSH, for example). When cygpath is
// unavailable, falls back to a native conversion that handles the
// common drive-letter case. UNC paths and other inputs without a drive
// letter return an error.
func WindowsSubsystemPathWithCygpath(ctx context.Context, cygpathExe, orig string) (string, error) {
	out, err := exec.CommandContext(ctx, cygpathExe, filepath.ToSlash(orig)).CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	logrus.WithError(err).Debugf("cygpath unavailable for %q (output: %q), attempting native conversion", orig, strings.TrimSpace(string(out)))
	// Only absolute drive-letter inputs have a well-defined Cygwin form
	// without consulting the working directory. Drive-relative inputs
	// (e.g. "C:foo") would silently turn into an unrelated absolute path,
	// so reject them.
	if filepath.IsAbs(orig) {
		if vol := filepath.VolumeName(orig); len(vol) == 2 && vol[1] == ':' {
			// We expect orig[2:] to begin with a separator for absolute
			// drive paths (C:\foo, C:/foo). Strip the leading slash
			// explicitly to keep the result canonical.
			tail := strings.TrimPrefix(filepath.ToSlash(orig[2:]), "/")
			converted := "/" + strings.ToLower(vol[:1]) + "/" + tail
			logrus.Debugf("native cygpath fallback: %q -> %q", orig, converted)
			return converted, nil
		}
	}
	return "", fmt.Errorf("cannot convert %q to a Cygwin-style path: cygpath unavailable and input is not an absolute drive-letter path", orig)
}

func WindowsSubsystemPathForLinux(ctx context.Context, orig, distro string) (string, error) {
	out, err := exec.CommandContext(ctx, "wsl", "-d", distro, "--exec", "wslpath", filepath.ToSlash(orig)).CombinedOutput()
	if err != nil {
		logrus.WithError(err).Errorf("failed to convert path to mingw, maybe wsl command is not operational?")
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
