// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package winpath is the PoC pure-Go replacement for the single cygpath.exe
// shell-out in Lima at pkg/ioutilx/ioutilx.go:54. It converts a Windows
// path (e.g. "C:\\Users\\me\\.lima") into the form expected by whichever
// SSH client Lima is about to invoke:
//
//   - Win32-OpenSSH         → native Windows path, slashes normalized.
//   - MSYS / MSYS2 / Git-Bash → "/c/Users/me/.lima"
//   - Cygwin                 → "/cygdrive/c/Users/me/.lima"
//
// Detection is environment-driven (MSYSTEM, CYGWIN, the resolved ssh
// binary path) so the choice matches what the SSH client itself would
// expect. Parsing is done with pure-string logic rather than
// `path/filepath`, so the package is testable on any host (filepath's
// Windows semantics are only active on GOOS=windows).
package winpath

import (
	"os"
	"strings"
)

// Style is which path namespace a downstream tool consumes.
type Style int

const (
	// StyleNative is a normal Windows path, slashes normalized.
	StyleNative Style = iota
	// StyleMSYS is MSYS / MSYS2 / Git-Bash style: "/c/Users/me".
	StyleMSYS
	// StyleCygwin is Cygwin style: "/cygdrive/c/Users/me".
	StyleCygwin
)

func (s Style) String() string {
	switch s {
	case StyleMSYS:
		return "msys"
	case StyleCygwin:
		return "cygwin"
	default:
		return "native"
	}
}

// Env is the subset of process environment + filesystem facts that affect
// detection. Pulled out so tests can drive every branch without mutating
// os.Environ. In production, use EnvFromOS().
type Env struct {
	// MSYSTEM is set by MSYS2 launchers (e.g. "MINGW64", "MSYS").
	MSYSTEM string
	// CYGWIN is set on Cygwin installations.
	CYGWIN string
	// SSH is Lima's override for the SSH binary path (matches the SSH
	// env var Lima already respects in pkg/sshutil).
	SSH string
	// LookPath resolves an executable name to a path on disk. Tests
	// stub this; production passes exec.LookPath.
	LookPath func(name string) (string, error)
}

// EnvFromOS reads the live process environment.
func EnvFromOS(lookPath func(string) (string, error)) Env {
	return Env{
		MSYSTEM:  os.Getenv("MSYSTEM"),
		CYGWIN:   os.Getenv("CYGWIN"),
		SSH:      os.Getenv("SSH"),
		LookPath: lookPath,
	}
}

// DetectStyle picks the path style that the SSH client Lima is about to
// invoke will accept. Order matters: explicit env vars win over binary
// path heuristics, because a user who exports MSYSTEM is asserting intent.
func DetectStyle(env Env) Style {
	if env.MSYSTEM != "" {
		return StyleMSYS
	}
	if env.CYGWIN != "" {
		return StyleCygwin
	}

	sshPath := env.SSH
	if sshPath == "" && env.LookPath != nil {
		if p, err := env.LookPath("ssh"); err == nil {
			sshPath = p
		}
	}
	if sshPath == "" {
		return StyleNative
	}

	low := strings.ToLower(toSlash(sshPath))
	switch {
	case strings.Contains(low, "/cygwin"):
		return StyleCygwin
	case strings.Contains(low, "/git/usr/bin/"),
		strings.Contains(low, "/msys64/"),
		strings.Contains(low, "/msys32/"),
		strings.Contains(low, "/mingw64/"),
		strings.Contains(low, "/mingw32/"):
		return StyleMSYS
	default:
		// Win32-OpenSSH typically lives under
		// C:\Windows\System32\OpenSSH\ssh.exe — the default branch.
		return StyleNative
	}
}

// toSlash replaces backslashes with forward slashes without touching the
// rest of the string. We deliberately do not call filepath.ToSlash because
// on Linux it is a no-op.
func toSlash(p string) string {
	return strings.ReplaceAll(p, `\`, `/`)
}

// windowsVolumeName mirrors filepath.VolumeName's behavior for Windows
// inputs, but works regardless of GOOS. It recognizes:
//
//   - drive letter:  "C:" / "c:"  → "C:"
//   - UNC prefix:    `\\server\share\...` or `//server/share/...` → `\\server\share`
//
// Anything else returns "".
func windowsVolumeName(p string) string {
	if len(p) >= 2 && p[1] == ':' && isDriveLetter(p[0]) {
		return p[:2]
	}
	// UNC: \\server\share  or  //server/share
	if len(p) >= 2 && (p[0] == '\\' || p[0] == '/') && (p[1] == '\\' || p[1] == '/') {
		// find next separator after the server name
		rest := p[2:]
		serverEnd := indexAnySep(rest)
		if serverEnd < 0 {
			return p
		}
		shareStart := serverEnd + 1
		shareRest := rest[shareStart:]
		shareEnd := indexAnySep(shareRest)
		if shareEnd < 0 {
			return p
		}
		return p[:2+shareStart+shareEnd]
	}
	return ""
}

func isDriveLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func indexAnySep(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' || s[i] == '/' {
			return i
		}
	}
	return -1
}

// Convert translates a Windows-style absolute path into the requested
// style. Relative paths and non-Windows paths are returned slash-
// normalized — matching what cygpath -u does as a no-op fallback.
//
// This is the body that replaces the cygpath.exe shell-out in
// ioutilx.WindowsSubsystemPath. No external process, no error from a
// missing cygpath binary, deterministic on every host.
func Convert(orig string, style Style) (string, error) {
	vol := windowsVolumeName(orig)

	// UNC: leave the structure, just normalize slashes. Neither MSYS nor
	// Cygwin rewrite UNC paths into the /cygdrive namespace; passing them
	// through is what `cygpath -u` does for UNC inputs.
	if strings.HasPrefix(vol, `\\`) || strings.HasPrefix(vol, `//`) {
		return toSlash(orig), nil
	}

	// No drive letter — return as-is (slash-normalized). The caller is
	// passing something we don't know how to remap.
	if len(vol) < 2 {
		return toSlash(orig), nil
	}

	drive := strings.ToLower(string(vol[0]))
	rest := toSlash(orig[len(vol):])
	if !strings.HasPrefix(rest, "/") {
		rest = "/" + rest
	}

	switch style {
	case StyleMSYS:
		return "/" + drive + rest, nil
	case StyleCygwin:
		return "/cygdrive/" + drive + rest, nil
	default:
		// Native: keep the drive letter, just normalize slashes.
		return strings.ToUpper(string(vol[0])) + ":" + rest, nil
	}
}

// WindowsSubsystemPath is the drop-in pure-Go replacement for
// ioutilx.WindowsSubsystemPath. The original signature took a
// context.Context to bound the cygpath subprocess; we keep an analogous
// signature shape so the call site at pkg/ioutilx/ioutilx.go:54 needs
// only a trivial edit. No syscall is performed.
func WindowsSubsystemPath(env Env, orig string) (string, error) {
	return Convert(orig, DetectStyle(env))
}
