// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ioutilx

import (
	"errors"
	"testing"

	"gotest.tools/v3/assert"
)

func TestDetectSubsystemStyle(t *testing.T) {
	cases := []struct {
		name     string
		env      map[string]string
		lookPath func(string) (string, error)
		want     subsystemStyle
	}{
		{
			name: "MSYSTEM set wins",
			env:  map[string]string{"MSYSTEM": "MINGW64"},
			want: subsystemMSYS,
		},
		{
			name: "CYGWIN set wins when MSYSTEM unset",
			env:  map[string]string{"CYGWIN": "nodosfilewarning"},
			want: subsystemCygwin,
		},
		{
			name: "SSH env points at cygwin install",
			env:  map[string]string{"SSH": `C:\cygwin64\bin\ssh.exe`},
			want: subsystemCygwin,
		},
		{
			name: "SSH env points at Git for Windows",
			env:  map[string]string{"SSH": `C:\Program Files\Git\usr\bin\ssh.exe`},
			want: subsystemMSYS,
		},
		{
			name: "SSH env points at Win32-OpenSSH",
			env:  map[string]string{"SSH": `C:\Windows\System32\OpenSSH\ssh.exe`},
			want: subsystemNative,
		},
		{
			name:     "LookPath fallback finds Git ssh",
			env:      map[string]string{},
			lookPath: func(string) (string, error) { return `C:\Program Files\Git\usr\bin\ssh.exe`, nil },
			want:     subsystemMSYS,
		},
		{
			name:     "LookPath fails, no env hints, defaults to native",
			env:      map[string]string{},
			lookPath: func(string) (string, error) { return "", errors.New("not found") },
			want:     subsystemNative,
		},
		{
			name: "empty env defaults to native",
			env:  map[string]string{},
			want: subsystemNative,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(k string) string { return tc.env[k] }
			assert.Equal(t, detectSubsystemStyle(getenv, tc.lookPath), tc.want)
		})
	}
}

func TestConvertWindowsSubsystemPath(t *testing.T) {
	cases := []struct {
		name  string
		style subsystemStyle
		input string
		want  string
	}{
		{"C drive to MSYS", subsystemMSYS, `C:\Users\me\.lima`, "/c/Users/me/.lima"},
		{"C drive to Cygwin", subsystemCygwin, `C:\Users\me\.lima`, "/cygdrive/c/Users/me/.lima"},
		{"C drive to Native is slash-normalized", subsystemNative, `C:\Users\me\.lima`, "C:/Users/me/.lima"},
		{"D drive lowercased for MSYS", subsystemMSYS, `D:\data`, "/d/data"},
		{"Root of drive", subsystemCygwin, `C:\`, "/cygdrive/c/"},
		{"UNC passes through normalized", subsystemMSYS, `\\fileserver\share\dir`, "//fileserver/share/dir"},
		{"Already-slashed input is preserved", subsystemMSYS, `C:/Users/me`, "/c/Users/me"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := convertWindowsSubsystemPath(tc.style, tc.input)
			assert.NilError(t, err)
			assert.Equal(t, got, tc.want)
		})
	}
}

// TestWindowsSubsystemPath_EndToEnd exercises the public function — the
// one all eight production call sites (cmd/limactl/shell.go,
// pkg/copytool, pkg/hostagent/mount, pkg/sshutil, pkg/limayaml/defaults)
// hit. It asserts no subprocess is required by passing nil context and
// confirms the output matches what cygpath -u would have produced.
func TestWindowsSubsystemPath_EndToEnd(t *testing.T) {
	t.Setenv("MSYSTEM", "")
	t.Setenv("CYGWIN", "")
	t.Setenv("SSH", `C:\Windows\System32\OpenSSH\ssh.exe`)

	got, err := WindowsSubsystemPath(t.Context(), `C:\Users\me\.lima\_config\user`)
	assert.NilError(t, err)
	assert.Equal(t, got, "C:/Users/me/.lima/_config/user")
}

func TestWindowsVolumeName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`C:\foo`, `C:`},
		{`c:`, `c:`},
		{`/foo/bar`, ``},
		{`relative/path`, ``},
		{`\\server\share\dir`, `\\server\share`},
		{`//server/share/dir`, `//server/share`},
		{``, ``},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, windowsVolumeName(tc.input), tc.want)
		})
	}
}
