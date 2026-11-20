// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

// rootOwnedExecutable returns a system binary that passes validateExecutablePath,
// or skips the test when the host has none.
func rootOwnedExecutable(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks("/usr/bin/true")
	if err != nil {
		t.Skipf("no /usr/bin/true: %v", err)
	}
	if err := validateExecutablePath(p); err != nil {
		t.Skipf("%q is not usable as a root-owned executable here: %v", p, err)
	}
	return p
}

func TestUsernetDaemon(t *testing.T) {
	exe := rootOwnedExecutable(t)

	userOwned := filepath.Join(t.TempDir(), "daemon")
	assert.NilError(t, os.WriteFile(userOwned, []byte("#!/bin/sh\n"), 0o755))

	cases := []struct {
		name       string
		network    Network
		want       string
		wantErr    string
		skipAsRoot bool
	}{
		{name: "built-in", network: Network{Mode: ModeUserV2}},
		{name: "external", network: Network{Mode: ModeUserV2, Daemon: exe}, want: exe},
		{name: "relative", network: Network{Mode: ModeUserV2, Daemon: "bin/daemon"}, wantErr: "not an absolute path"},
		{name: "missing", network: Network{Mode: ModeUserV2, Daemon: "/nonexistent/daemon"}, wantErr: "no such file"},
		{name: "directory", network: Network{Mode: ModeUserV2, Daemon: filepath.Dir(exe)}, wantErr: "not an executable regular file"},
		{name: "wrong mode", network: Network{Mode: ModeShared, Daemon: exe}, wantErr: "only supported for mode"},
		{name: "not root-owned", network: Network{Mode: ModeUserV2, Daemon: userOwned}, wantErr: "is not owned by", skipAsRoot: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipAsRoot && os.Geteuid() == 0 {
				t.Skip("every file is effectively root-owned when running as root")
			}
			cfg := Config{Networks: map[string]Network{"test": tc.network}}
			got, err := cfg.UsernetDaemon("test")
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestUsernetDaemonUndefinedNetwork(t *testing.T) {
	_, err := (&Config{}).UsernetDaemon("test")
	assert.ErrorContains(t, err, "is not defined")
}
