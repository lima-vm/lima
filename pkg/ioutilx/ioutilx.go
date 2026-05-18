// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ioutilx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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

// WindowsSubsystemPath converts a Windows path into the form expected by
// whichever SSH client Lima is about to invoke. It replaces a former
// cygpath.exe subprocess; the conversion is now pure-Go and deterministic,
// so it no longer depends on Cygwin/MSYS2 being installed on the host.
//
// Three target styles are picked from the environment:
//   - Win32-OpenSSH (default)        → native path, slashes normalized.
//   - MSYS2 / Git-Bash (MSYSTEM set) → "/c/Users/me/...".
//   - Cygwin       (CYGWIN set)      → "/cygdrive/c/Users/me/...".
//
// The ctx parameter is accepted for signature compatibility with the
// previous subprocess-based implementation; no syscall is performed.
func WindowsSubsystemPath(_ context.Context, orig string) (string, error) {
	return convertWindowsSubsystemPath(detectSubsystemStyle(os.Getenv, exec.LookPath), orig)
}

// subsystemStyle is the target SSH-client path namespace.
type subsystemStyle int

const (
	subsystemNative subsystemStyle = iota // Win32-OpenSSH
	subsystemMSYS                         // MSYS2 / MSYS / Git-Bash
	subsystemCygwin                       // Cygwin
)

// detectSubsystemStyle picks the style the downstream ssh binary expects.
// Order: explicit env vars first (user-asserted intent), then a heuristic
// over the resolved ssh binary path. getenv and lookPath are injected so
// tests can drive every branch without touching the real environment.
func detectSubsystemStyle(getenv func(string) string, lookPath func(string) (string, error)) subsystemStyle {
	if getenv("MSYSTEM") != "" {
		return subsystemMSYS
	}
	if getenv("CYGWIN") != "" {
		return subsystemCygwin
	}
	sshPath := getenv("SSH")
	if sshPath == "" && lookPath != nil {
		if p, err := lookPath("ssh"); err == nil {
			sshPath = p
		}
	}
	if sshPath == "" {
		return subsystemNative
	}
	low := strings.ToLower(strings.ReplaceAll(sshPath, `\`, `/`))
	switch {
	case strings.Contains(low, "/cygwin"):
		return subsystemCygwin
	case strings.Contains(low, "/git/usr/bin/"),
		strings.Contains(low, "/msys64/"),
		strings.Contains(low, "/msys32/"),
		strings.Contains(low, "/mingw64/"),
		strings.Contains(low, "/mingw32/"):
		return subsystemMSYS
	default:
		// Win32-OpenSSH typically lives under
		// C:\Windows\System32\OpenSSH\ssh.exe.
		return subsystemNative
	}
}

// convertWindowsSubsystemPath translates an absolute Windows-style path
// into the requested style. Pure string logic, no path/filepath calls, so
// this is testable on any host (filepath's Windows semantics only kick in
// when GOOS=windows). UNC inputs pass through with slashes normalized,
// matching cygpath -u's behavior for UNC paths.
func convertWindowsSubsystemPath(style subsystemStyle, orig string) (string, error) {
	vol := windowsVolumeName(orig)

	// UNC (\\server\share\...): preserve structure, normalize slashes.
	if strings.HasPrefix(vol, `\\`) || strings.HasPrefix(vol, `//`) {
		return strings.ReplaceAll(orig, `\`, `/`), nil
	}

	// Not a drive-letter path: return as-is (slash-normalized).
	if len(vol) < 2 {
		return strings.ReplaceAll(orig, `\`, `/`), nil
	}

	drive := strings.ToLower(string(vol[0]))
	rest := strings.ReplaceAll(orig[len(vol):], `\`, `/`)
	if !strings.HasPrefix(rest, "/") {
		rest = "/" + rest
	}

	switch style {
	case subsystemMSYS:
		return "/" + drive + rest, nil
	case subsystemCygwin:
		return "/cygdrive/" + drive + rest, nil
	default:
		return strings.ToUpper(string(vol[0])) + ":" + rest, nil
	}
}

// windowsVolumeName mirrors filepath.VolumeName's behavior for Windows
// inputs, but works regardless of GOOS. Recognizes drive letters ("C:")
// and UNC prefixes (\\server\share). Returns "" for anything else.
func windowsVolumeName(p string) string {
	if len(p) >= 2 && p[1] == ':' &&
		((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z')) {
		return p[:2]
	}
	if len(p) >= 2 && (p[0] == '\\' || p[0] == '/') && (p[1] == '\\' || p[1] == '/') {
		rest := p[2:]
		serverEnd := strings.IndexAny(rest, `\/`)
		if serverEnd < 0 {
			return p
		}
		shareRest := rest[serverEnd+1:]
		shareEnd := strings.IndexAny(shareRest, `\/`)
		if shareEnd < 0 {
			return p
		}
		return p[:2+serverEnd+1+shareEnd]
	}
	return ""
}

func WindowsSubsystemPathForLinux(ctx context.Context, orig, distro string) (string, error) {
	out, err := exec.CommandContext(ctx, "wsl", "-d", distro, "--exec", "wslpath", filepath.ToSlash(orig)).CombinedOutput()
	if err != nil {
		logrus.WithError(err).Errorf("failed to convert path to mingw, maybe wsl command is not operational?")
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
