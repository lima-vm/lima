//go:build darwin

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
	"strings"
	"syscall"

	"github.com/balajiv113/fd"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/sudoers"
)

// SudoOpenBlockDeviceCommand is the hidden helper that opens a host block
// device as root and passes the descriptor back to the unprivileged process.
const SudoOpenBlockDeviceCommand = "sudo-open-block-device"

// ServeSudoOpenBlockDevice runs inside the short-lived privileged helper
// process launched via sudo. It receives a JSON request on stdin, opens the
// requested host device node as root, and sends the resulting file descriptor
// back to the already-running unprivileged process over a private Unix socket.
func ServeSudoOpenBlockDevice(r io.Reader) error {
	var req sudoOpenBlockDeviceRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return fmt.Errorf("failed to decode request: %w", err)
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
// file descriptor to the unprivileged caller via SCM_RIGHTS.
type sudoOpenBlockDeviceRequest struct {
	DevicePath string `json:"devicePath"`
	SocketPath string `json:"socketPath"`
}

// Sudoers returns the sudoers fragment needed to run the hidden block-device
// helper without prompting.
func Sudoers(group string) (string, error) {
	helperArgs, err := sudoOpenBlockDeviceHelperArgs()
	if err != nil {
		return "", err
	}
	return sudoers.NOPASSWD("%"+group, "root", "wheel", strings.Join(helperArgs, " ")), nil
}

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

// Open creates a private Unix socket, asks the privileged helper to open the
// host device and connect back to that socket, then returns a duplicated
// descriptor that the caller can retain for as long as needed.
func Open(ctx context.Context, devicePath, socketPath string) (*os.File, error) {
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
	cmd := sudoers.NewCommand(ctx, "root", "wheel", &stdin, &stdout, &stderr, "", helperArgs...)
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
