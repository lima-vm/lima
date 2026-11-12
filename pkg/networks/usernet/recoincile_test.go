// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usernet

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/networks"
)

// TestStartFailsWhenDaemonExits: a daemon that exits before creating its
// sockets must fail Start instead of leaving it waiting forever.
func TestStartFailsWhenDaemonExits(t *testing.T) {
	cfg, err := networks.LoadConfig()
	assert.NilError(t, err)
	if _, err := cfg.UsernetDaemon("ext"); err != nil {
		t.Skipf("%q is not usable as a root-owned daemon here: %v", exitingDaemon, err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	err = Start(ctx, "ext")
	assert.ErrorContains(t, err, "exited before it was ready")
	assert.ErrorContains(t, err, "exit status 1")
}

// TestStartRefusesOtherRunningDaemon: a running daemon is reused only if it is
// the one the network is configured with, so that changing `daemon` cannot
// leave an instance on the previous daemon.
func TestStartRefusesOtherRunningDaemon(t *testing.T) {
	cfg, err := networks.LoadConfig()
	assert.NilError(t, err)
	if _, err := cfg.UsernetDaemon("ext"); err != nil {
		t.Skipf("%q is not usable as a root-owned daemon here: %v", exitingDaemon, err)
	}
	tests := []struct {
		network string
		running string // the recorded daemon; "-" for no record
		wantErr bool
	}{
		{network: "user-v2", running: "-"},
		{network: "user-v2", running: ""},
		{network: "user-v2", running: exitingDaemon, wantErr: true},
		{network: "ext", running: exitingDaemon},
		{network: "ext", running: "-", wantErr: true},
		{network: "ext", running: "/usr/local/bin/other", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.network+"/"+tc.running, func(t *testing.T) {
			pidFile, err := PIDFile(tc.network)
			assert.NilError(t, err)
			dir := filepath.Dir(pidFile)
			assert.NilError(t, os.MkdirAll(dir, 0o755))
			// This process stands in for the running daemon.
			assert.NilError(t, os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644))
			t.Cleanup(func() { _ = os.Remove(pidFile) })
			record := filepath.Join(dir, "usernet_daemon")
			_ = os.Remove(record)
			if tc.running != "-" {
				assert.NilError(t, os.WriteFile(record, []byte(tc.running), 0o644))
			}

			err = Start(t.Context(), tc.network)
			if tc.wantErr {
				assert.ErrorContains(t, err, "is not the one the network is configured with")
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
