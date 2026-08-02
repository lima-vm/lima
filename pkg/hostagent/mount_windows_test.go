// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

// TestMountPathAndSftpServer: the mount path stays native whatever the
// toolchain, and the sftp-server comes from the toolchain, except under
// builtin. A Cygwin path form would reach the server as backslashes a Cygwin
// ssh.exe has eaten, so the Cygwin cases here assert the native form too.
func TestMountPathAndSftpServer(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only path handling")
	}
	ctx := t.Context()

	const location = `C:\Users\lima\shared`
	const wantPath = "C:/Users/lima/shared"

	// nativeToolchain writes an ssh.exe, plus a sibling sftp-server.exe when
	// withSftpServer, and returns both paths. The production helpers expand
	// the 8.3 short path (C:\Users\RUNNER~1\...) that t.TempDir() returns on
	// GitHub Windows runners, so this fixture expands it too.
	nativeToolchain := func(t *testing.T, withSftpServer bool) (sshExe, sftpExe string) {
		t.Helper()
		dir := t.TempDir()
		resolved, err := filepath.EvalSymlinks(dir)
		assert.NilError(t, err)
		sshExe = filepath.Join(resolved, "ssh.exe")
		assert.NilError(t, os.WriteFile(sshExe, nil, 0o644))
		if withSftpServer {
			sftpExe = filepath.Join(resolved, "sftp-server.exe")
			assert.NilError(t, os.WriteFile(sftpExe, nil, 0o644))
		}
		return sshExe, sftpExe
	}

	t.Run("sibling sftp-server pairs with the native path form", func(t *testing.T) {
		sshExe, sftpExe := nativeToolchain(t, true)

		gotPath, gotServer := mountPathAndSftpServer(ctx, sshutil.SSHExe{Exe: sshExe}, "", location)
		assert.Equal(t, gotPath, wantPath)
		assert.Equal(t, gotServer, sftpExe)
	})

	t.Run("no sibling sftp-server leaves detection to sshocker", func(t *testing.T) {
		sshExe, _ := nativeToolchain(t, false)

		gotPath, gotServer := mountPathAndSftpServer(ctx, sshutil.SSHExe{Exe: sshExe}, "", location)
		assert.Equal(t, gotPath, wantPath)
		assert.Equal(t, gotServer, "", "no sibling -> sshocker detects one itself")
	})

	t.Run("builtin driver gets no sftp-server", func(t *testing.T) {
		sshExe, _ := nativeToolchain(t, true)

		gotPath, gotServer := mountPathAndSftpServer(ctx, sshutil.SSHExe{Exe: sshExe}, limatype.SFTPDriverBuiltin, location)
		assert.Equal(t, gotPath, wantPath)
		assert.Equal(t, gotServer, "", "builtin serves the mount in-process")
	})

	t.Run("a Cygwin toolchain gets the native path under every driver", func(t *testing.T) {
		sshExe, _ := nativeToolchain(t, false)
		// cygpath.exe beside ssh.exe marks the toolchain as Cygwin.
		assert.NilError(t, os.WriteFile(filepath.Join(filepath.Dir(sshExe), "cygpath.exe"), nil, 0o644))

		for _, driver := range []string{"", limatype.SFTPDriverBuiltin, limatype.SFTPDriverOpenSSHSFTPServer} {
			gotPath, _ := mountPathAndSftpServer(ctx, sshutil.SSHExe{Exe: sshExe}, driver, location)
			assert.Equal(t, gotPath, wantPath, "driver %q: sshocker turns a Cygwin form into backslashes that a Cygwin ssh.exe eats", driver)
		}
	})

	t.Run("explicit openssh-sftp-server driver pairs with the sibling", func(t *testing.T) {
		sshExe, sftpExe := nativeToolchain(t, true)

		gotPath, gotServer := mountPathAndSftpServer(ctx, sshutil.SSHExe{Exe: sshExe}, limatype.SFTPDriverOpenSSHSFTPServer, location)
		assert.Equal(t, gotPath, wantPath)
		assert.Equal(t, gotServer, sftpExe)
	})
}
