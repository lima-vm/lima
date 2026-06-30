//go:build darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/osutil"
)

func TestSudoersForUserAndHelper(t *testing.T) {
	sudoers := sudoersForUserAndHelper("alice", []string{"/usr/local/bin/limactl", SudoOpenBlockDeviceCommand, "/dev/rdisk2"})
	assert.Equal(t, sudoers, "alice ALL=(root:wheel) NOPASSWD:NOSETENV: /usr/local/bin/limactl "+SudoOpenBlockDeviceCommand+" /dev/rdisk2\n")
	assert.Assert(t, !contains(sudoers, "%everyone"))
}

func TestSudoersForUserAndHelpers(t *testing.T) {
	sudoers := sudoersForUserAndHelpers(
		"alice",
		[]string{"/usr/local/bin/limactl", SudoOpenBlockDeviceCommand, "/dev/rdisk2"},
		[]string{"/usr/local/bin/limactl", SudoOpenBlockDeviceCommand, "/dev/rdisk3"},
	)
	assert.Equal(t, sudoers, "alice ALL=(root:wheel) NOPASSWD:NOSETENV: \\\n"+
		"    /usr/local/bin/limactl "+SudoOpenBlockDeviceCommand+" /dev/rdisk2, \\\n"+
		"    /usr/local/bin/limactl "+SudoOpenBlockDeviceCommand+" /dev/rdisk3\n")
}

func TestValidateSudoersUserName(t *testing.T) {
	assert.NilError(t, validateSudoersUserName("alice"))

	testCases := []string{
		"",
		"%everyone",
		"#501",
		"alice admin",
		"alice:admin",
		"alice=admin",
		`DOMAIN\alice`,
	}
	for _, userName := range testCases {
		t.Run(userName, func(t *testing.T) {
			assert.Assert(t, validateSudoersUserName(userName) != nil)
		})
	}
}

func TestSudoOpenBlockDeviceRequestValidate(t *testing.T) {
	valid := sudoOpenBlockDeviceRequest{
		DevicePath: "/dev/disk4",
		SocketPath: "/tmp/block-device.0.sock",
		Nonce:      strings.Repeat("a", blockDeviceNonceHexLen),
	}
	assert.NilError(t, valid.validate())

	testCases := []struct {
		name          string
		request       sudoOpenBlockDeviceRequest
		errorContains string
	}{
		{
			name: "empty device path",
			request: sudoOpenBlockDeviceRequest{
				SocketPath: valid.SocketPath,
				Nonce:      valid.Nonce,
			},
			errorContains: "devicePath must not be empty",
		},
		{
			name: "relative device path",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: "disk4",
				SocketPath: valid.SocketPath,
				Nonce:      valid.Nonce,
			},
			errorContains: "must be an absolute path",
		},
		{
			name: "raw disk device path",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: "/dev/rdisk4s1",
				SocketPath: valid.SocketPath,
				Nonce:      valid.Nonce,
			},
		},
		{
			name: "non disk device path",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: "/dev/null",
				SocketPath: valid.SocketPath,
				Nonce:      valid.Nonce,
			},
			errorContains: "must be a macOS disk device path",
		},
		{
			name: "fd device path",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: "/dev/fd/3",
				SocketPath: valid.SocketPath,
				Nonce:      valid.Nonce,
			},
			errorContains: "must be a macOS disk device path",
		},
		{
			name: "non dev path",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: "/tmp/disk4",
				SocketPath: valid.SocketPath,
				Nonce:      valid.Nonce,
			},
			errorContains: "must be a macOS disk device path",
		},
		{
			name: "disk path without number",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: "/dev/disk",
				SocketPath: valid.SocketPath,
				Nonce:      valid.Nonce,
			},
			errorContains: "must be a macOS disk device path",
		},
		{
			name: "disk path with nested suffix",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: "/dev/disk4/secret",
				SocketPath: valid.SocketPath,
				Nonce:      valid.Nonce,
			},
			errorContains: "must be a macOS disk device path",
		},
		{
			name: "unnormalized socket path",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: valid.DevicePath,
				SocketPath: "/tmp/../tmp/block-device.0.sock",
				Nonce:      valid.Nonce,
			},
			errorContains: "socketPath",
		},
		{
			name: "relative socket path",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: valid.DevicePath,
				SocketPath: "block-device.0.sock",
				Nonce:      valid.Nonce,
			},
			errorContains: "must be an absolute path",
		},
		{
			name: "missing nonce",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: valid.DevicePath,
				SocketPath: valid.SocketPath,
			},
			errorContains: "nonce",
		},
		{
			name: "non hex nonce",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: valid.DevicePath,
				SocketPath: valid.SocketPath,
				Nonce:      strings.Repeat("g", blockDeviceNonceHexLen),
			},
			errorContains: "nonce",
		},
		{
			name: "short nonce",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: valid.DevicePath,
				SocketPath: valid.SocketPath,
				Nonce:      strings.Repeat("a", blockDeviceNonceHexLen-1),
			},
			errorContains: "nonce",
		},
		{
			name: "socket path too long",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: valid.DevicePath,
				SocketPath: "/tmp/" + strings.Repeat("a", osutil.UnixPathMax),
				Nonce:      valid.Nonce,
			},
			errorContains: "UNIX_PATH_MAX",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.request.validate()
			if tc.errorContains == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.errorContains)
			}
		})
	}
}

func TestSudoOpenBlockDeviceRequestValidateForAllowedDevice(t *testing.T) {
	valid := sudoOpenBlockDeviceRequest{
		DevicePath: "/dev/rdisk4",
		SocketPath: "/tmp/block-device.0.sock",
		Nonce:      strings.Repeat("a", blockDeviceNonceHexLen),
	}

	assert.NilError(t, valid.validateForAllowedDevice("/dev/rdisk4"))

	err := valid.validateForAllowedDevice("/dev/rdisk5")
	assert.ErrorContains(t, err, `devicePath "/dev/rdisk4" is not allowed`)

	err = valid.validateForAllowedDevice("/dev/null")
	assert.ErrorContains(t, err, "allowed devicePath")
}

func TestServeSudoOpenBlockDeviceRejectsDeviceOutsideAllowlist(t *testing.T) {
	req := `{"devicePath":"/dev/rdisk4","socketPath":"/tmp/block-device.0.sock","nonce":"` + strings.Repeat("a", blockDeviceNonceHexLen) + `"}`

	err := ServeSudoOpenBlockDevice("/dev/rdisk5", strings.NewReader(req))
	assert.ErrorContains(t, err, `devicePath "/dev/rdisk4" is not allowed`)
}

func TestGenerateNonce(t *testing.T) {
	nonce1, err := generateNonce()
	assert.NilError(t, err)
	nonce2, err := generateNonce()
	assert.NilError(t, err)
	assert.Assert(t, nonceRE.MatchString(nonce1))
	assert.Assert(t, nonceRE.MatchString(nonce2))
	assert.Assert(t, nonce1 != nonce2)
}

func TestValidateDiskDeviceMode(t *testing.T) {
	assert.NilError(t, validateDiskDeviceMode("/dev/disk4", os.ModeDevice))
	assert.NilError(t, validateDiskDeviceMode("/dev/disk4s1", os.ModeDevice))
	assert.NilError(t, validateDiskDeviceMode("/dev/disk4s1s1", os.ModeDevice))
	assert.NilError(t, validateDiskDeviceMode("/dev/rdisk4", os.ModeDevice|os.ModeCharDevice))
	assert.NilError(t, validateDiskDeviceMode("/dev/rdisk4s1", os.ModeDevice|os.ModeCharDevice))
	assert.NilError(t, validateDiskDeviceMode("/dev/rdisk4s1s1", os.ModeDevice|os.ModeCharDevice))

	testCases := []struct {
		name          string
		path          string
		mode          os.FileMode
		errorContains string
	}{
		{
			name:          "regular file",
			path:          "/dev/disk4",
			mode:          0o600,
			errorContains: "not a device node",
		},
		{
			name:          "arbitrary character device",
			path:          "/dev/null",
			mode:          os.ModeDevice | os.ModeCharDevice,
			errorContains: "must be a macOS disk device path",
		},
		{
			name:          "block path is character device",
			path:          "/dev/disk4",
			mode:          os.ModeDevice | os.ModeCharDevice,
			errorContains: "must be a block disk device",
		},
		{
			name:          "raw path is block device",
			path:          "/dev/rdisk4",
			mode:          os.ModeDevice,
			errorContains: "must be a raw character disk device",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDiskDeviceMode(tc.path, tc.mode)
			assert.ErrorContains(t, err, tc.errorContains)
		})
	}
}

func TestValidateDiskDeviceFileRejectsRegularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "not-a-disk")
	assert.NilError(t, err)
	defer f.Close()

	err = validateDiskDeviceFile("/dev/disk4", f)
	assert.ErrorContains(t, err, "not a device node")
}

func TestSudoOriginalUID(t *testing.T) {
	t.Setenv("SUDO_UID", "501")
	uid, err := sudoOriginalUID()
	assert.NilError(t, err)
	assert.Equal(t, uid, uint32(501))

	t.Setenv("SUDO_UID", "")
	_, err = sudoOriginalUID()
	assert.ErrorContains(t, err, "SUDO_UID is not set")

	t.Setenv("SUDO_UID", "not-a-uid")
	_, err = sudoOriginalUID()
	assert.ErrorContains(t, err, "invalid SUDO_UID")
}

func TestValidatePrivateSocketPath(t *testing.T) {
	privateDir := shortTempDir(t)
	uid := uint32(os.Getuid())

	assert.NilError(t, validatePrivateSocketPath(filepath.Join(privateDir, "block-device.sock"), uid))

	publicDir := filepath.Join(shortTempDir(t), "public")
	assert.NilError(t, os.Mkdir(publicDir, 0o755))
	err := validatePrivateSocketPath(filepath.Join(publicDir, "block-device.sock"), uid)
	assert.ErrorContains(t, err, "must not be accessible by group or other users")

	err = validatePrivateSocketPath(filepath.Join(privateDir, "block-device.sock"), uid+1)
	assert.ErrorContains(t, err, "is not owned by sudo user")

	linkParent := filepath.Join(shortTempDir(t), "link")
	assert.NilError(t, os.Symlink(privateDir, linkParent))
	err = validatePrivateSocketPath(filepath.Join(linkParent, "block-device.sock"), uid)
	assert.ErrorContains(t, err, "must not be a symlink")

	fileParent := filepath.Join(shortTempDir(t), "not-dir")
	assert.NilError(t, os.WriteFile(fileParent, nil, 0o600))
	err = validatePrivateSocketPath(filepath.Join(fileParent, "block-device.sock"), uid)
	assert.ErrorContains(t, err, "is not a directory")

	err = validatePrivateSocketPath(filepath.Join(privateDir, strings.Repeat("a", osutil.UnixPathMax)), uid)
	assert.ErrorContains(t, err, "UNIX_PATH_MAX")
}

func TestRemoveStaleSocket(t *testing.T) {
	privateDir := shortTempDir(t)

	missing := filepath.Join(privateDir, "missing.sock")
	assert.NilError(t, removeStaleSocket(missing))

	socketPath := filepath.Join(privateDir, "stale.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	assert.NilError(t, err)
	listener.SetUnlinkOnClose(false)
	assert.NilError(t, listener.Close())
	assert.NilError(t, removeStaleSocket(socketPath))
	_, err = os.Lstat(socketPath)
	assert.Assert(t, os.IsNotExist(err))

	regularPath := filepath.Join(privateDir, "regular.sock")
	assert.NilError(t, os.WriteFile(regularPath, nil, 0o600))
	err = removeStaleSocket(regularPath)
	assert.ErrorContains(t, err, "not a Unix socket")

	linkPath := filepath.Join(privateDir, "link.sock")
	assert.NilError(t, os.Symlink(regularPath, linkPath))
	err = removeStaleSocket(linkPath)
	assert.ErrorContains(t, err, "symlink")
}

func TestUnixPeerUID(t *testing.T) {
	privateDir := shortTempDir(t)
	socketPath := filepath.Join(privateDir, "peer.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	assert.NilError(t, err)
	defer listener.Close()

	errCh := make(chan error, 1)
	go func() {
		conn, err := listener.AcceptUnix()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		errCh <- validateUnixPeerUID(conn, uint32(os.Getuid()))
	}()

	client, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socketPath, Net: "unix"})
	assert.NilError(t, err)
	defer client.Close()
	assert.NilError(t, <-errCh)
}

func TestUnixPeerUIDRejectsUnexpectedUID(t *testing.T) {
	privateDir := shortTempDir(t)
	socketPath := filepath.Join(privateDir, "peer.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	assert.NilError(t, err)
	defer listener.Close()

	errCh := make(chan error, 1)
	go func() {
		conn, err := listener.AcceptUnix()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		errCh <- validateUnixPeerUID(conn, uint32(os.Getuid()+1))
	}()

	client, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socketPath, Net: "unix"})
	assert.NilError(t, err)
	defer client.Close()
	assert.ErrorContains(t, <-errCh, "expected uid")
}

func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "lima-bd-test-") //nolint:usetesting // Unix socket paths must stay below macOS UNIX_PATH_MAX.
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	assert.NilError(t, os.Chmod(dir, 0o700))
	return dir
}

func TestValidateSecurePathElement(t *testing.T) {
	const rootUID = 0
	const rootGID = 0

	assert.NilError(t, validateSecurePathElement("/usr/local/bin/limactl", fakeFileInfo{
		mode: 0o755,
		stat: &syscall.Stat_t{Uid: rootUID, Gid: rootGID},
	}, rootUID, rootGID))
	assert.NilError(t, validateSecurePathElement("/usr/local/bin", fakeFileInfo{
		mode: os.ModeDir | 0o775,
		stat: &syscall.Stat_t{Uid: rootUID, Gid: rootGID},
	}, rootUID, rootGID))

	testCases := []struct {
		name          string
		info          os.FileInfo
		errorContains string
	}{
		{
			name: "symlink",
			info: fakeFileInfo{
				mode: os.ModeSymlink | 0o777,
				stat: &syscall.Stat_t{Uid: rootUID, Gid: rootGID},
			},
			errorContains: "symlink",
		},
		{
			name: "not root owned",
			info: fakeFileInfo{
				mode: 0o755,
				stat: &syscall.Stat_t{Uid: 501, Gid: rootGID},
			},
			errorContains: "not owned by root",
		},
		{
			name: "group writable by non-root group",
			info: fakeFileInfo{
				mode: 0o775,
				stat: &syscall.Stat_t{Uid: rootUID, Gid: 20},
			},
			errorContains: "group-writable",
		},
		{
			name: "world writable",
			info: fakeFileInfo{
				mode: 0o777,
				stat: &syscall.Stat_t{Uid: rootUID, Gid: rootGID},
			},
			errorContains: "world-writable",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSecurePathElement("/usr/local/bin/limactl", tc.info, rootUID, rootGID)
			assert.ErrorContains(t, err, tc.errorContains)
		})
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

type fakeFileInfo struct {
	mode os.FileMode
	stat any
}

func (f fakeFileInfo) Name() string       { return "limactl" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return f.stat }
