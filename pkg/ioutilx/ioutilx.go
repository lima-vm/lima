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

func WindowsSubsystemPath(ctx context.Context, orig string) (string, error) {
	out, err := exec.CommandContext(ctx, "cygpath", filepath.ToSlash(orig)).CombinedOutput()
	if err != nil {
		logrus.WithError(err).Errorf("failed to convert path to mingw, maybe not using Git ssh?")
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func WindowsSubsystemPathForLinux(ctx context.Context, orig, distro string) (string, error) {
	out, err := exec.CommandContext(ctx, "wsl", "-d", distro, "--exec", "wslpath", filepath.ToSlash(orig)).CombinedOutput()
	if err != nil {
		logrus.WithError(err).Errorf("failed to convert path to mingw, maybe wsl command is not operational?")
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// TranslateWindowsToWSLPath translates a Windows path (e.g., C:\Users\...) to a WSL-compliant path (e.g., /mnt/c/Users/...)
func TranslateWindowsToWSLPath(orig string) (string, error) {
	cleaned := orig
	// Trim trailing slashes/backslashes, but keep drive root if length <= 2
	for len(cleaned) > 2 && (cleaned[len(cleaned)-1] == '/' || cleaned[len(cleaned)-1] == '\\') {
		cleaned = cleaned[:len(cleaned)-1]
	}

	// UNC paths check
	if strings.HasPrefix(cleaned, `\\`) || strings.HasPrefix(orig, `//`) {
		return orig, fmt.Errorf("UNC paths are not supported for WSL translation: %q", orig)
	}

	// Recognizable drive letter paths like C: or C:\path or C:/path
	if len(cleaned) >= 2 && cleaned[1] == ':' && ((cleaned[0] >= 'A' && cleaned[0] <= 'Z') || (cleaned[0] >= 'a' && cleaned[0] <= 'z')) {
		drive := strings.ToLower(string(cleaned[0]))
		if len(cleaned) == 2 {
			return "/mnt/" + drive, nil
		}
		// Must be followed by a slash or backslash
		if cleaned[2] == '/' || cleaned[2] == '\\' {
			rest := strings.ReplaceAll(cleaned[2:], "\\", "/")
			return "/mnt/" + drive + rest, nil
		}
	}

	return orig, fmt.Errorf("not an absolute Windows path with drive letter: %q", orig)
}
