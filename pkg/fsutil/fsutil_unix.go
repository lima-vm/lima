//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fsutil

import (
	"context"
	"errors"
)

// errWindowsPathConversionNotSupported is returned by the Windows path
// conversion stubs below. Callers only invoke them behind a
// runtime.GOOS == "windows" check, so this is never actually returned in
// practice; it exists so the fsutil_windows.go API still compiles here.
var errWindowsPathConversionNotSupported = errors.New("fsutil: Windows path conversion is not supported on this platform")

// WindowsSubsystemPath is the non-Windows stub for the Windows-only
// implementation in fsutil_windows.go.
func WindowsSubsystemPath(_ context.Context, _ string) (string, error) {
	return "", errWindowsPathConversionNotSupported
}

// WindowsSubsystemPathWithCygpath is the non-Windows stub for the
// Windows-only implementation in fsutil_windows.go.
func WindowsSubsystemPathWithCygpath(_ context.Context, _, _ string) (string, error) {
	return "", errWindowsPathConversionNotSupported
}

// WindowsSubsystemPathForLinux is the non-Windows stub for the Windows-only
// implementation in fsutil_windows.go.
func WindowsSubsystemPathForLinux(_ context.Context, _, _ string) (string, error) {
	return "", errWindowsPathConversionNotSupported
}
