//go:build darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/osutil"
)

func TestSudoersForUserAndHelpers(t *testing.T) {
	single := sudoersForUserAndHelpers("alice", []string{"/usr/local/libexec/lima/privileged/" + PrivilegedHelperName, "/dev/rdisk2"})
	assert.Equal(t, single, "alice ALL=(root:wheel) NOPASSWD:NOSETENV: /usr/local/libexec/lima/privileged/"+PrivilegedHelperName+" /dev/rdisk2\n")
	assert.Assert(t, !strings.Contains(single, "%everyone"))

	sudoers := sudoersForUserAndHelpers(
		"alice",
		[]string{"/usr/local/libexec/lima/privileged/" + PrivilegedHelperName, "/dev/rdisk2"},
		[]string{"/usr/local/libexec/lima/privileged/" + PrivilegedHelperName, "/dev/rdisk3"},
	)
	assert.Equal(t, sudoers, "alice ALL=(root:wheel) NOPASSWD:NOSETENV: \\\n"+
		"    /usr/local/libexec/lima/privileged/"+PrivilegedHelperName+" /dev/rdisk2, \\\n"+
		"    /usr/local/libexec/lima/privileged/"+PrivilegedHelperName+" /dev/rdisk3\n")
}

func TestSudoersGrantOrderAndDuplicates(t *testing.T) {
	a := []string{"/opt/lima/libexec/lima/privileged/" + PrivilegedHelperName, "/dev/disk4"}
	b := []string{a[0], "/dev/disk5"}
	want := sudoersForUserAndHelpers("alice", a, b)
	assert.Equal(t, sudoersForUserAndHelpers("alice", b, a, b), want)
	assert.Assert(t, sudoersForUserAndHelpers("alice", a) != want)
	assert.DeepEqual(t, a, []string{b[0], "/dev/disk4"})
}

func TestPrivilegedHelperPathForExecutable(t *testing.T) {
	path := privilegedHelperPathForExecutable("/opt/lima/bin/limactl")

	assert.Equal(t, path, "/opt/lima/libexec/lima/privileged/"+PrivilegedHelperName)
	assert.Assert(t, !strings.Contains(path, "/bin/limactl"))
}

func TestPrivilegedHelperPathExplainsMissingInstallation(t *testing.T) {
	_, err := PrivilegedHelperPath(filepath.Join(t.TempDir(), "bin", "limactl"))
	assert.ErrorContains(t, err, "missing or not securely installed")
	assert.ErrorContains(t, err, "/opt/lima/bin/limactl")
	assert.ErrorContains(t, err, "No source build is required")
	assert.ErrorContains(t, err, "https://lima-vm.io/docs/config/disk/#sudoers-setup")
}

func TestPrivateSocketRejectsSymlink(t *testing.T) {
	dir := shortTempDir(t)
	outside := filepath.Join(shortTempDir(t), "outside.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: outside, Net: "unix"})
	assert.NilError(t, err)
	defer listener.Close()
	link := filepath.Join(dir, "helper.sock")
	assert.NilError(t, os.Symlink(outside, link))
	assert.ErrorContains(t, validatePrivateSocketPath(link, uint32(os.Getuid())), "symlink")
}

func TestValidateExecutable(t *testing.T) {
	assert.NilError(t, ValidateExecutable("/opt/lima/bin/limactl"))
	assert.ErrorContains(t, ValidateExecutable("/opt/lima/libexec/lima/lima-driver-vz"), "external VZ")
}

func TestDialUnixPeerAuthenticatesBeforeSending(t *testing.T) {
	for _, matches := range []bool{true, false} {
		t.Run(strconv.FormatBool(matches), func(t *testing.T) {
			path := filepath.Join(shortTempDir(t), "listener.sock")
			listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
			assert.NilError(t, err)
			defer listener.Close()
			uid := uint32(os.Getuid())
			if !matches {
				uid++
			}
			client, err := dialUnixPeer(path, uid)
			if matches {
				assert.NilError(t, err)
				assert.NilError(t, client.Close())
			} else {
				assert.ErrorContains(t, err, "expected uid")
				assert.Assert(t, client == nil)
			}
			server, err := listener.AcceptUnix()
			assert.NilError(t, err)
			defer server.Close()
			assert.NilError(t, server.SetReadDeadline(time.Now().Add(time.Second)))
			var data [1]byte
			n, err := server.Read(data[:])
			assert.Equal(t, n, 0)
			assert.Error(t, err, "EOF")
		})
	}
}

func TestValidateSecurePathElementRejectsMissingStat(t *testing.T) {
	assert.ErrorContains(t, validateSecurePathElement("/helper", fakeFileInfo{}, 0, 0), "stat buffer")
}

func TestValidateSecureHelperFile(t *testing.T) {
	assert.NilError(t, validateSecureHelperFile("/helper", fakeFileInfo{mode: 0o755}))
	assert.ErrorContains(t, validateSecureHelperFile("/helper", fakeFileInfo{mode: os.ModeDir | 0o755}), "regular file")
	assert.ErrorContains(t, validateSecureHelperFile("/helper", fakeFileInfo{mode: 0o644}), "executable")
}

func TestDiskDeviceIdentity(t *testing.T) {
	expected := fakeFileInfo{mode: os.ModeDevice, stat: &syscall.Stat_t{Rdev: 4}}
	assert.NilError(t, validateDiskDeviceIdentity("/dev/disk4", expected, expected))
	other := fakeFileInfo{mode: os.ModeDevice, stat: &syscall.Stat_t{Rdev: 5}}
	assert.ErrorContains(t, validateDiskDeviceIdentity("/dev/disk4", other, expected), "does not identify")
	assert.ErrorContains(t, validateDiskDeviceIdentity("/dev/disk4", fakeFileInfo{}, expected), "does not identify")
	assert.ErrorContains(t, validateDiskDeviceIdentity("/dev/disk4", expected, fakeFileInfo{mode: os.ModeSymlink}), "not a device")
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

func TestWaitForReceivedFDHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	file, err := waitForReceivedFD(ctx, make(chan receivedFD))
	assert.Assert(t, file == nil)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestDuplicateFilePreservesCloseOnExec(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "fd")
	assert.NilError(t, err)
	defer f.Close()

	dup, err := duplicateFileCloseOnExec(f, "fd")
	assert.NilError(t, err)
	defer dup.Close()

	flags, err := unix.FcntlInt(dup.Fd(), unix.F_GETFD, 0)
	assert.NilError(t, err)
	assert.Assert(t, flags&unix.FD_CLOEXEC != 0)
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
	// WHY: umask can remove these bits; this fixture must stay world-accessible.
	assert.NilError(t, os.Chmod(publicDir, 0o755))
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

func TestValidateSecurePathACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "limactl")
	assert.NilError(t, os.WriteFile(path, nil, 0o755))
	fi, err := os.Lstat(path)
	assert.NilError(t, err)

	assert.NilError(t, validateSecurePathACL(path, fi))

	err = exec.CommandContext(t.Context(), "chmod", "+a", "everyone allow write", path).Run()
	if err != nil {
		t.Skipf("failed to add test ACL: %v", err)
	}

	err = validateSecurePathACL(path, fi)
	assert.ErrorContains(t, err, "extended ACL entries")
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

func TestValidateSecurePathRejectsSudoersMetacharacters(t *testing.T) {
	// Reject syntax before inspecting the filesystem: no root-owned fixture is needed.
	for _, character := range " \t\r\n,:=\\*?[]!#^" {
		t.Run(fmt.Sprintf("%U", character), func(t *testing.T) {
			path := "/opt/lima" + string(character) + "bin/limactl"
			assert.ErrorContains(t, validateSecurePath(path, 0, 0), "sudoers metacharacters")
		})
	}
}
