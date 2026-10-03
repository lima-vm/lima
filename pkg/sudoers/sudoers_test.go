// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sudoers

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestReadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lima")
	assert.NilError(t, os.WriteFile(path, []byte("content\n"), 0o600))

	content, ok, err := ReadFile(path)
	assert.NilError(t, err)
	assert.Assert(t, ok)
	assert.Equal(t, content, "content\n")

	_, _, err = ReadFile(filepath.Join(t.TempDir(), "missing"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestReadFileToleratesUnreadableSudoersFile(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("the file mode does not keep the caller out")
	}
	path := filepath.Join(t.TempDir(), "lima")
	assert.NilError(t, os.WriteFile(path, []byte("content\n"), 0o000))

	content, ok, err := ReadFile(path)
	assert.NilError(t, err)
	assert.Assert(t, !ok)
	assert.Equal(t, content, "")
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

func TestContainsActiveFragmentRejectsBroadenedGrant(t *testing.T) {
	grant := NOPASSWD("alice", "root", "wheel", "/bin/mkdir /var/run/lima")
	for _, file := range []string{
		strings.TrimSpace(grant) + ", /bin/sh\n",
		"other" + grant,
		strings.TrimSpace(grant) + "-other\n",
	} {
		assert.Assert(t, !ContainsActiveFragment(file, grant), "accepted broadened grant: %s", file)
	}
	assert.Assert(t, ContainsActiveFragment("# header\n"+grant+"bob ALL=(root) /bin/true\n", grant))
}

func TestContainsActiveFragmentLineContinuations(t *testing.T) {
	grant := NOPASSWD("alice", "root", "wheel", "/bin/mkdir /var/run/lima", "/bin/rmdir /var/run/lima")
	for _, tc := range []struct {
		name, prefix string
		want         bool
	}{
		{"ordinary line", "Defaults env_keep += FOO\n", true},
		{"continued line", "Defaults env_keep += FOO \\\n", false},
		{"continued line before comment", "Defaults env_keep += FOO \\\n# comment\n", true},
		{"commented continuation", "# comment \\\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, ContainsActiveFragment(tc.prefix+grant, grant), tc.want)
			// Check the regression against sudo's parser, not just our matcher.
			visudo, err := exec.LookPath("visudo")
			if err != nil {
				visudo = "/usr/sbin/visudo"
				if _, err := os.Stat(visudo); err != nil {
					t.Skip("visudo is not installed")
				}
			}
			file := filepath.Join(t.TempDir(), "sudoers")
			assert.NilError(t, os.WriteFile(file, []byte(tc.prefix+grant), 0o600))
			output, err := exec.CommandContext(t.Context(), visudo, "-c", "-f", file).CombinedOutput()
			assert.Equal(t, err == nil, tc.want, "%s", output)
		})
	}
	assert.Assert(t, !ContainsActiveFragment("\n", "# no grants\n"))
}
