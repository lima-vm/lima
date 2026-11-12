// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usernet

import (
	"context"
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
