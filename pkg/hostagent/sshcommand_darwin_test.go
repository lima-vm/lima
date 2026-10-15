// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestSSHCommandCapsNofile(t *testing.T) {
	adjustNofileRlimit()
	var limit syscall.Rlimit
	assert.NilError(t, syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit))
	out, err := sshCommand(t.Context(), "/bin/sh", "-c", "ulimit -S -n").Output()
	assert.NilError(t, err)
	assert.Equal(t, strings.TrimSpace(string(out)), strconv.FormatUint(min(limit.Max, 1024), 10))
}

func TestSSHCommandIgnoresSSHInWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "ssh"), []byte("#!/bin/sh\n"), 0o755))
	t.Chdir(dir)
	t.Setenv("PATH", ".")
	assert.ErrorIs(t, sshCommand(t.Context(), "ssh").Run(), exec.ErrDot)
}

// fakeSSHCheckingNofile raises the fd limit as the hostagent does, then puts an ssh
// on PATH that fails, printing its soft fd limit, unless that limit is at most 1024.
func fakeSSHCheckingNofile(t *testing.T) {
	adjustNofileRlimit()
	dir := t.TempDir()
	script := `#!/bin/sh
n=$(ulimit -S -n)
[ "$n" != unlimited ] && [ "$n" -le 1024 ] || { echo "soft fd limit $n"; exit 1; }
`
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestWaitForRequirementCapsNofile(t *testing.T) {
	fakeSSHCheckingNofile(t)
	a := &HostAgent{instConfig: &limatype.LimaYAML{OS: new(limatype.LINUX)}, sshConfig: &ssh.SSHConfig{}}
	assert.NilError(t, a.waitForRequirement(requirement{description: "fd limit", script: "#!/bin/sh\n"}))
}

func TestForwardSSHCapsNofile(t *testing.T) {
	fakeSSHCheckingNofile(t)
	assert.NilError(t, forwardSSH(t.Context(), &ssh.SSHConfig{}, "127.0.0.1", 22, "127.0.0.1:1", "127.0.0.1:2", verbForward, false))
}
