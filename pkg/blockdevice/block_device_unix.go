//go:build unix

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/balajiv113/fd"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/sudoers"
)

// ServeSudoOpenBlockDevice runs inside the short-lived privileged helper
// process launched via sudo. It receives a JSON request on stdin, opens the
// requested host device node as root, and sends the resulting file descriptor
// back to the already-running unprivileged process over a private Unix socket.
//
// The descriptor must travel over a Unix socket via SCM_RIGHTS because there
// is no inheritance path back to the caller: the helper is a child of sudo,
// and a child cannot pass descriptors to an already-running parent through
// fork/exec inheritance. Escalating only this tiny helper, instead of any VM
// process, is what keeps the rest of Lima rootless even when the device node
// itself is only openable by root.
func ServeSudoOpenBlockDevice(r io.Reader) error {
	var req sudoOpenBlockDeviceRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return fmt.Errorf("failed to decode request: %w", err)
	}
	if req.Probe {
		// A probe request only checks that the helper can be launched through
		// sudo without a password; it must not touch any device.
		return nil
	}
	if err := req.validate(); err != nil {
		return err
	}

	deviceFile, err := os.OpenFile(req.DevicePath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open %q: %w", req.DevicePath, err)
	}
	defer deviceFile.Close()

	fi, err := deviceFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat %q: %w", req.DevicePath, err)
	}
	if fi.Mode()&os.ModeDevice == 0 {
		return fmt.Errorf("%q is not a device node", req.DevicePath)
	}

	if req.MakeAccessible {
		// Hand the device node itself to the invoking user, for VM backends
		// that must open the device by path in a separate unprivileged
		// process and have no way to accept an inherited descriptor. This is
		// the same access grant an administrator would perform by hand with
		// `sudo chown`; only the owner changes, so the mode bits and group
		// access stay as the OS configured them. The uid is taken from sudo's
		// own environment, never from the request, so the helper cannot be
		// used to grant device access to anyone but the user who invoked it.
		sudoUID := os.Getenv("SUDO_UID")
		uid, err := strconv.Atoi(sudoUID)
		if err != nil {
			return fmt.Errorf("failed to parse SUDO_UID %q: %w", sudoUID, err)
		}
		if err := os.Chown(req.DevicePath, uid, -1); err != nil {
			return fmt.Errorf("failed to chown %q to uid %d: %w", req.DevicePath, uid, err)
		}
		return nil
	}

	socketAddr, err := net.ResolveUnixAddr("unix", req.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to resolve socket %q: %w", req.SocketPath, err)
	}
	conn, err := net.DialUnix("unix", nil, socketAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to socket %q: %w", req.SocketPath, err)
	}
	defer conn.Close()

	if err := fd.Put(conn, deviceFile); err != nil {
		return fmt.Errorf("failed to send file descriptor for %q: %w", req.DevicePath, err)
	}
	return nil
}

// sudoOpenBlockDeviceRequest is the JSON payload sent to the privileged helper.
// SocketPath is the absolute path to the private Unix socket that the helper
// must connect back to after opening DevicePath, so it can return the opened
// file descriptor to the unprivileged caller via SCM_RIGHTS. A request with
// MakeAccessible set instead changes the ownership of the device node to the
// invoking user. A request with Probe set verifies only that the helper can
// be launched without a password.
type sudoOpenBlockDeviceRequest struct {
	DevicePath     string `json:"devicePath,omitempty"`
	SocketPath     string `json:"socketPath,omitempty"`
	MakeAccessible bool   `json:"makeAccessible,omitempty"`
	Probe          bool   `json:"probe,omitempty"`
}

// Sudoers returns the sudoers fragment needed to run the hidden block-device
// helper without prompting. The subject is a sudoers user specification,
// e.g. a user name or "%group".
func Sudoers(subject string) (string, error) {
	helperArgs, err := sudoOpenBlockDeviceHelperArgs()
	if err != nil {
		return "", err
	}
	return sudoers.NOPASSWD(subject, "root", sudoers.RootGroup(), strings.Join(helperArgs, " ")), nil
}

// validate constrains what the helper will touch as root. The /dev prefix
// and normalization requirements exist because the helper runs with a
// NOPASSWD sudoers entry: without them, that entry would amount to a generic
// open-or-chown-any-file-as-root primitive for everyone the entry covers.
func (r sudoOpenBlockDeviceRequest) validate() error {
	if r.DevicePath == "" {
		return errors.New("devicePath must not be empty")
	}
	if !filepath.IsAbs(r.DevicePath) {
		return fmt.Errorf("devicePath %q must be an absolute path", r.DevicePath)
	}
	if filepath.Clean(r.DevicePath) != r.DevicePath {
		return fmt.Errorf("devicePath %q must be normalized", r.DevicePath)
	}
	if !strings.HasPrefix(r.DevicePath, "/dev/") {
		return fmt.Errorf("devicePath %q must be under /dev", r.DevicePath)
	}

	if r.MakeAccessible {
		// MakeAccessible operates on the device node itself; no socket is
		// involved.
		if r.SocketPath != "" {
			return errors.New("socketPath must be empty for a makeAccessible request")
		}
		return nil
	}
	if r.SocketPath == "" {
		return errors.New("socketPath must not be empty")
	}
	if !filepath.IsAbs(r.SocketPath) {
		return fmt.Errorf("socketPath %q must be an absolute path", r.SocketPath)
	}
	if filepath.Clean(r.SocketPath) != r.SocketPath {
		return fmt.Errorf("socketPath %q must be normalized", r.SocketPath)
	}
	return nil
}

func sudoOpenBlockDeviceHelperArgs() ([]string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if strings.ContainsAny(exe, " \t\r\n") {
		return nil, fmt.Errorf("limactl executable path %q contains whitespace and cannot be used in sudoers", exe)
	}
	return []string{exe, SudoOpenBlockDeviceCommand}, nil
}

// EnsureAccess verifies upfront that every configured host block device can
// be opened, prompting once for the user's sudo password when the privileged
// helper will be needed. It must run in a foreground process that still owns
// the terminal: the hostagent runs in a background process group, where a
// sudo password prompt would be stopped by SIGTTIN before the user ever sees
// it. The sudo timestamp cached by `sudo --validate` here is what lets the
// hostagent's non-interactive sudo invocation succeed moments later.
func EnsureAccess(ctx context.Context, devicePaths []string) error {
	if len(devicePaths) == 0 {
		return nil
	}
	var inaccessible []string
	for _, devicePath := range devicePaths {
		deviceFile, err := openDirect(devicePath)
		if err == nil {
			_ = deviceFile.Close()
			continue
		}
		logrus.Debugf("Cannot open block device %q directly (%v); the privileged helper will be used", devicePath, err)
		inaccessible = append(inaccessible, devicePath)
	}
	if len(inaccessible) == 0 {
		return nil
	}
	hint := fmt.Sprintf("run `%s sudoers | sudo tee /etc/sudoers.d/lima` once to allow passwordless access, or grant your user direct access to the device node (e.g. membership in its owning group)", os.Args[0])
	if !helperRunsWithoutPassword(ctx) {
		if !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd()) {
			return fmt.Errorf("block devices %v require sudo to open, but no terminal is available for the password prompt (hint: %s)", inaccessible, hint)
		}
		logrus.Infof("Attaching block devices %v requires root; sudo may now ask for your password", inaccessible)
		if err := sudoers.CacheCredentials(ctx); err != nil {
			return fmt.Errorf("cannot open block devices %v: %w", inaccessible, err)
		}
		// Confirm that the cached credentials actually let the helper run
		// non-interactively, instead of letting the VM boot and fail later in
		// the background hostagent, where sudo cannot prompt.
		if !helperRunsWithoutPassword(ctx) {
			return fmt.Errorf("sudo credentials were cached, but the block-device helper still cannot run non-interactively (hint: %s)", hint)
		}
	}
	// Open each device through the real helper path once, so device-level
	// failures (e.g. SIP-protected disks on macOS that even root cannot
	// open) surface here instead of later in the background hostagent.
	socketDir, err := os.MkdirTemp("", "lima-block-device-probe")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(socketDir) }()
	for i, devicePath := range inaccessible {
		deviceFile, err := Open(ctx, devicePath, filepath.Join(socketDir, fmt.Sprintf("probe.%d.sock", i)))
		if err != nil {
			return fmt.Errorf("cannot open block device %q: %w", devicePath, err)
		}
		_ = deviceFile.Close()
	}
	return nil
}

// helperRunsWithoutPassword reports whether the privileged helper can be
// launched through sudo without prompting, either via a NOPASSWD sudoers rule
// (e.g. installed with `limactl sudoers`) or via a still-valid cached sudo
// timestamp. It runs the real helper with a no-op probe request, because the
// answer must reflect the exact argv that the NOPASSWD rule covers
// (`sudo --list` only answers whether a command is permitted at all, not
// whether it needs a password).
func helperRunsWithoutPassword(ctx context.Context) bool {
	helperArgs, err := sudoOpenBlockDeviceHelperArgs()
	if err != nil {
		return false
	}
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(sudoOpenBlockDeviceRequest{Probe: true}); err != nil {
		return false
	}
	cmd := sudoers.NewCommand(ctx, "root", sudoers.RootGroup(), &stdin, nil, nil, "", helperArgs...)
	logrus.Debugf("Probing passwordless sudo for the block-device helper: %v", cmd.Args)
	return cmd.Run() == nil
}

// EnsureDeviceAccessible makes sure the current user can open the host device
// read-write by path, for VM backends that run as a separate unprivileged
// process and cannot accept an inherited descriptor (QEMU and krunkit on
// macOS: /dev/fd/N re-checks the device node permissions there, and fcntl
// F_SETFL fails with ENOTTY on disk descriptors, which breaks QEMU's
// /dev/fdset duplication). When direct access is denied, the privileged
// helper changes the ownership of the device node to the invoking user, the
// same thing an administrator would do by hand; devfs nodes revert when the
// device is detached.
func EnsureDeviceAccessible(ctx context.Context, devicePath string) error {
	deviceFile, err := openDirect(devicePath)
	if err == nil {
		_ = deviceFile.Close()
		return nil
	}
	logrus.Debugf("Cannot open block device %q directly (%v); asking the privileged helper to make it accessible", devicePath, err)

	helperArgs, err := sudoOpenBlockDeviceHelperArgs()
	if err != nil {
		return err
	}
	req := sudoOpenBlockDeviceRequest{
		DevicePath:     devicePath,
		MakeAccessible: true,
	}
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(req); err != nil {
		return err
	}
	var stdout, stderr bytes.Buffer
	cmd := sudoers.NewCommand(ctx, "root", sudoers.RootGroup(), &stdin, &stdout, &stderr, "", helperArgs...)
	logrus.Debugf("Making block device %q accessible via sudo helper: %v", devicePath, cmd.Args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %v: stdout=%q, stderr=%q: %w", cmd.Args, stdout.String(), stderr.String(), err)
	}

	// Ownership alone does not guarantee access: the mode bits may still deny
	// the owner (e.g. a 0000 node), so prove the grant worked while a clear
	// error can still be returned to the user.
	deviceFile, err = openDirect(devicePath)
	if err != nil {
		return fmt.Errorf("block device %q is still not accessible after the privileged helper changed its ownership: %w", devicePath, err)
	}
	_ = deviceFile.Close()
	return nil
}

// openDirect opens the host device without privilege escalation. Trying this
// first means users who already have access to the device node (user-owned
// ramdisks on macOS, members of the "disk" group on Linux) never trigger sudo
// at all. The device-node check mirrors the privileged helper's, so the two
// paths accept exactly the same set of targets.
func openDirect(devicePath string) (*os.File, error) {
	deviceFile, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	fi, err := deviceFile.Stat()
	if err != nil {
		_ = deviceFile.Close()
		return nil, err
	}
	if fi.Mode()&os.ModeDevice == 0 {
		_ = deviceFile.Close()
		return nil, fmt.Errorf("%q is not a device node", devicePath)
	}
	return deviceFile, nil
}

// Open returns a retained read-write descriptor for the host device. It first
// tries to open the device directly with the caller's own privileges, and
// only falls back to the sudo helper when that fails: it then creates a
// private Unix socket, asks the privileged helper to open the host device and
// connect back to that socket, and returns a duplicated descriptor that the
// caller can retain for as long as needed.
func Open(ctx context.Context, devicePath, socketPath string) (*os.File, error) {
	deviceFile, err := openDirect(devicePath)
	if err == nil {
		logrus.Debugf("Opened block device %q directly without the sudo helper", devicePath)
		return deviceFile, nil
	}
	logrus.Debugf("Failed to open block device %q directly, falling back to the sudo helper: %v", devicePath, err)

	helperArgs, err := sudoOpenBlockDeviceHelperArgs()
	if err != nil {
		return nil, err
	}
	if err := os.RemoveAll(socketPath); err != nil {
		return nil, err
	}

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %q: %w", socketPath, err)
	}
	listener.SetUnlinkOnClose(true)
	defer func() {
		_ = listener.Close()
		_ = os.RemoveAll(socketPath)
	}()

	req := sudoOpenBlockDeviceRequest{
		DevicePath: devicePath,
		SocketPath: socketPath,
	}
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(req); err != nil {
		return nil, err
	}

	type receivedFD struct {
		file *os.File
		err  error
	}
	fdCh := make(chan receivedFD, 1)
	go func() {
		conn, err := listener.AcceptUnix()
		if err != nil {
			fdCh <- receivedFD{err: err}
			return
		}
		defer conn.Close()

		files, err := fd.Get(conn, 1, []string{filepath.Base(devicePath)})
		if err != nil {
			fdCh <- receivedFD{err: err}
			return
		}
		if len(files) != 1 {
			for _, file := range files {
				_ = file.Close()
			}
			fdCh <- receivedFD{err: fmt.Errorf("expected 1 file descriptor for %q, got %d", devicePath, len(files))}
			return
		}
		fdCh <- receivedFD{file: files[0]}
	}()

	var stdout, stderr bytes.Buffer
	cmd := sudoers.NewCommand(ctx, "root", sudoers.RootGroup(), &stdin, &stdout, &stderr, "", helperArgs...)
	logrus.Debugf("Opening block device %q via sudo helper: %v", devicePath, cmd.Args)
	if err := cmd.Run(); err != nil {
		_ = listener.Close()
		received := <-fdCh
		if received.file != nil {
			_ = received.file.Close()
		}
		return nil, fmt.Errorf("failed to run %v: stdout=%q, stderr=%q: %w", cmd.Args, stdout.String(), stderr.String(), err)
	}

	received := <-fdCh
	if received.err != nil {
		return nil, fmt.Errorf("failed to receive file descriptor for %q: %w", devicePath, received.err)
	}
	dupFD, err := syscall.Dup(int(received.file.Fd()))
	if err != nil {
		_ = received.file.Close()
		return nil, fmt.Errorf("failed to duplicate file descriptor for %q: %w", devicePath, err)
	}
	_ = received.file.Close()

	return os.NewFile(uintptr(dupFD), filepath.Base(devicePath)), nil
}
