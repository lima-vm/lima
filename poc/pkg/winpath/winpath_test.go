// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package winpath

import (
	"errors"
	"testing"
)

func TestDetectStyle(t *testing.T) {
	cases := []struct {
		name string
		env  Env
		want Style
	}{
		{
			name: "MSYSTEM set wins",
			env:  Env{MSYSTEM: "MINGW64"},
			want: StyleMSYS,
		},
		{
			name: "CYGWIN set wins when MSYSTEM unset",
			env:  Env{CYGWIN: "nodosfilewarning"},
			want: StyleCygwin,
		},
		{
			name: "SSH env points at cygwin install",
			env:  Env{SSH: `C:\cygwin64\bin\ssh.exe`},
			want: StyleCygwin,
		},
		{
			name: "SSH env points at Git for Windows",
			env:  Env{SSH: `C:\Program Files\Git\usr\bin\ssh.exe`},
			want: StyleMSYS,
		},
		{
			name: "SSH env points at Win32-OpenSSH",
			env:  Env{SSH: `C:\Windows\System32\OpenSSH\ssh.exe`},
			want: StyleNative,
		},
		{
			name: "LookPath fallback finds Git ssh",
			env: Env{
				LookPath: func(string) (string, error) {
					return `C:\Program Files\Git\usr\bin\ssh.exe`, nil
				},
			},
			want: StyleMSYS,
		},
		{
			name: "LookPath fails, no env hints → native",
			env: Env{
				LookPath: func(string) (string, error) {
					return "", errors.New("not found")
				},
			},
			want: StyleNative,
		},
		{
			name: "empty env → native",
			env:  Env{},
			want: StyleNative,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectStyle(tc.env)
			if got != tc.want {
				t.Fatalf("DetectStyle = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestConvert(t *testing.T) {
	cases := []struct {
		name  string
		input string
		style Style
		want  string
	}{
		{
			name:  "C drive to MSYS",
			input: `C:\Users\me\.lima`,
			style: StyleMSYS,
			want:  "/c/Users/me/.lima",
		},
		{
			name:  "C drive to Cygwin",
			input: `C:\Users\me\.lima`,
			style: StyleCygwin,
			want:  "/cygdrive/c/Users/me/.lima",
		},
		{
			name:  "C drive to Native is just slash-normalized",
			input: `C:\Users\me\.lima`,
			style: StyleNative,
			want:  "C:/Users/me/.lima",
		},
		{
			name:  "D drive lowercased",
			input: `D:\data`,
			style: StyleMSYS,
			want:  "/d/data",
		},
		{
			name:  "Root of drive",
			input: `C:\`,
			style: StyleCygwin,
			want:  "/cygdrive/c/",
		},
		{
			name:  "UNC path passes through normalized",
			input: `\\fileserver\share\dir`,
			style: StyleMSYS,
			want:  "//fileserver/share/dir",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Convert(tc.input, tc.style)
			if err != nil {
				t.Fatalf("Convert error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Convert(%q, %s) = %q, want %q", tc.input, tc.style, got, tc.want)
			}
		})
	}
}

// TestWindowsSubsystemPath_EndToEnd exercises the full public entry point
// — the one a maintainer would wire into pkg/ioutilx in place of the
// cygpath shell-out. It asserts that the drop-in replacement produces
// exactly the string each downstream SSH client would accept.
func TestWindowsSubsystemPath_EndToEnd(t *testing.T) {
	const input = `C:\Users\me\.lima\_config\user`

	cases := []struct {
		name string
		env  Env
		want string
	}{
		{
			name: "Git-Bash user → MSYS form",
			env:  Env{MSYSTEM: "MINGW64"},
			want: "/c/Users/me/.lima/_config/user",
		},
		{
			name: "Cygwin user → cygdrive form",
			env:  Env{CYGWIN: "nodosfilewarning"},
			want: "/cygdrive/c/Users/me/.lima/_config/user",
		},
		{
			name: "Win32-OpenSSH (default) → native path",
			env:  Env{SSH: `C:\Windows\System32\OpenSSH\ssh.exe`},
			want: "C:/Users/me/.lima/_config/user",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := WindowsSubsystemPath(tc.env, input)
			if err != nil {
				t.Fatalf("WindowsSubsystemPath error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
