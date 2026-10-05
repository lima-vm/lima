// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"gotest.tools/v3/assert"
)

const testScript = "#!/bin/sh\ntrue\n"

// fakeSSH prepends a fake ssh binary to PATH so executeScript runs it instead
// of the real ssh client.
func fakeSSH(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestExecuteScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell to fake the ssh binary")
	}
	fakeSSH(t, "exit 0")
	_, _, err := executeScript(t.Context(), "127.0.0.1", 0, &ssh.SSHConfig{}, testScript, "test")
	assert.NilError(t, err)
}

func TestExecuteScriptTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell to fake the ssh binary")
	}
	fakeSSH(t, "exec sleep 30")
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := executeScript(ctx, "127.0.0.1", 0, &ssh.SSHConfig{}, testScript, "test")
	assert.Assert(t, err != nil)
	assert.Assert(t, errors.Is(err, context.DeadlineExceeded), "unexpected error: %v", err)
	assert.Assert(t, time.Since(start) < 10*time.Second, "the ssh process was not killed by the context")
}
