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
	assert.Equal(t, NOPASSWD("%everyone", "root", "wheel", "/bin/mkdir -m 775 -p /private/var/run/lima"),
		"%everyone ALL=(root:wheel) NOPASSWD:NOSETENV: /bin/mkdir -m 775 -p /private/var/run/lima\n")

	assert.Equal(t, NOPASSWD("%everyone", "daemon", "staff", "/bin/start", "/bin/stop"),
		"%everyone ALL=(daemon:staff) NOPASSWD:NOSETENV: \\\n    /bin/start, \\\n    /bin/stop\n")
}

func TestContainsActiveFragmentIgnoresCommentsOnBothSides(t *testing.T) {
	file := "%admin ALL=(root:wheel) NOPASSWD:NOSETENV: /bin/mkdir -m 775 -p /private/var/run/lima\n" +
		"\n# Manage \"shared\" network daemons\n\n" +
		"%admin ALL=(root:wheel) NOPASSWD:NOSETENV: /opt/socket_vmnet/bin/socket_vmnet ...\n"
	fragment := "%admin ALL=(root:wheel) NOPASSWD:NOSETENV: /bin/mkdir -m 775 -p /private/var/run/lima\n" +
		"\n# Manage \"shared\" network daemons\n"

	assert.Assert(t, ContainsActiveFragment(file, fragment))
	assert.Assert(t, !ContainsActiveFragment("# "+strings.ReplaceAll(fragment, "\n", "\n# "), fragment))
}
