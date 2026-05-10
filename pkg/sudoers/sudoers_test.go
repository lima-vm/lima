// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sudoers

import (
	"io"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestArgs(t *testing.T) {
	assert.DeepEqual(t, Args("root", "wheel", "true"), []string{
		"--user", "root",
		"--group", "wheel",
		"--non-interactive",
		"true",
	})
}

func TestNewCommand(t *testing.T) {
	stdin := strings.NewReader("")
	cmd := NewCommand(t.Context(), "root", "wheel", stdin, io.Discard, io.Discard, "/tmp", "true")

	assert.Equal(t, cmd.Args[0], "sudo")
	assert.DeepEqual(t, cmd.Args[1:], Args("root", "wheel", "true"))
	assert.Equal(t, cmd.Stdin, stdin)
	assert.Equal(t, cmd.Stdout, io.Discard)
	assert.Equal(t, cmd.Stderr, io.Discard)
	assert.Equal(t, cmd.Dir, "/tmp")
}

func TestNOPASSWD(t *testing.T) {
	assert.Equal(t, NOPASSWD("%everyone", "root", "wheel", "/usr/local/bin/limactl sudo-open-block-device"),
		"%everyone ALL=(root:wheel) NOPASSWD:NOSETENV: /usr/local/bin/limactl sudo-open-block-device\n")

	assert.Equal(t, NOPASSWD("%everyone", "daemon", "staff", "/bin/start", "/bin/stop"),
		"%everyone ALL=(daemon:staff) NOPASSWD:NOSETENV: \\\n    /bin/start, \\\n    /bin/stop\n")
}
