//go:build darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/balajiv113/fd"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/sudoers"
)

// SudoOpenBlockDeviceCommand is the hidden helper that opens a host block
// device as root and passes the descriptor back to the unprivileged process.
const SudoOpenBlockDeviceCommand = "sudo-open-block-device"

const (
	blockDeviceNonceHexLen = 64
	helperReadTimeout      = 30 * time.Second
)

var (
	macOSDiskDevicePathRE = regexp.MustCompile(`^/dev/r?disk\d+(s\d+)*$`)
	nonceRE               = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// ServeSudoOpenBlockDevice runs inside the short-lived privileged helper
// process launched via sudo. It receives a JSON request on stdin, opens the
// requested host device node as root, and sends the resulting file descriptor
// back to the already-running unprivileged process over a private Unix socket.
func ServeSudoOpenBlockDevice(allowedDevicePath string, r io.Reader) error {
	var req sudoOpenBlockDeviceRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return fmt.Errorf("failed to decode request: %w", err)
	}
	if err := req.validateForAllowedDevice(allowedDevicePath); err != nil {
		return err
	}

	if err := validateDiskDeviceNode(req.DevicePath); err != nil {
		return err
	}
	originalUID, err := sudoOriginalUID()
	if err != nil {
		return err
	}
	if err := validatePrivateSocketPath(req.SocketPath, originalUID); err != nil {
		return err
	}
	deviceFile, err := openDiskDeviceNoFollow(req.DevicePath)
	if err != nil {
		return fmt.Errorf("failed to open %q: %w", req.DevicePath, err)
	}
	defer deviceFile.Close()

	fi, err := deviceFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat %q: %w", req.DevicePath, err)
	}
	if err := validateDiskDeviceMode(req.DevicePath, fi.Mode()); err != nil {
		return err
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

	if n, err := io.WriteString(conn, req.Nonce); err != nil {
		return fmt.Errorf("failed to send authentication nonce: %w", err)
	} else if n != len(req.Nonce) {
		return fmt.Errorf("failed to send authentication nonce: %w", io.ErrShortWrite)
	}
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
	Nonce      string `json:"nonce"`
}

type receivedFD struct {
	file *os.File
	err  error
}

// Sudoers returns the sudoers fragment needed to run the hidden block-device
// helper without prompting for the current user. The helper is intentionally
// scoped to a single user instead of the network group, because opening host
// disks gives direct access to host storage.
func Sudoers(devicePaths []string) (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return SudoersForUser(u.Username, devicePaths)
}

func SudoersForUser(userName string, devicePaths []string) (string, error) {
	if err := validateSudoersUserName(userName); err != nil {
		return "", err
	}
	if len(devicePaths) == 0 {
		return "", errors.New("at least one block device path is required")
	}
	var helperCommands [][]string
	for _, devicePath := range devicePaths {
		helperArgs, err := sudoOpenBlockDeviceHelperArgs(devicePath)
		if err != nil {
			return "", err
		}
		helperCommands = append(helperCommands, helperArgs)
	}
	return sudoersForUserAndHelpers(userName, helperCommands...), nil
}

func sudoersForUserAndHelper(userName string, helperArgs []string) string {
	return sudoers.NOPASSWD(userName, "root", "wheel", strings.Join(helperArgs, " "))
}

func sudoersForUserAndHelpers(userName string, helperArgs ...[]string) string {
	helperCommands := make([]string, 0, len(helperArgs))
	for _, args := range helperArgs {
		helperCommands = append(helperCommands, strings.Join(args, " "))
	}
	return sudoers.NOPASSWD(userName, "root", "wheel", helperCommands...)
}

func (r sudoOpenBlockDeviceRequest) validate() error {
	if err := validateDiskDevicePath(r.DevicePath); err != nil {
		return err
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
	if len(r.SocketPath) >= osutil.UnixPathMax {
		return fmt.Errorf("socketPath %q must be less than UNIX_PATH_MAX=%d characters, but is %d", r.SocketPath, osutil.UnixPathMax, len(r.SocketPath))
	}
	if !nonceRE.MatchString(r.Nonce) {
		return fmt.Errorf("nonce must be %d lowercase hexadecimal characters", blockDeviceNonceHexLen)
	}
	return nil
}

func (r sudoOpenBlockDeviceRequest) validateForAllowedDevice(allowedDevicePath string) error {
	if err := validateDiskDevicePath(allowedDevicePath); err != nil {
		return fmt.Errorf("allowed devicePath: %w", err)
	}
	if err := r.validate(); err != nil {
		return err
	}
	if r.DevicePath != allowedDevicePath {
		return fmt.Errorf("devicePath %q is not allowed by sudoers entry for %q", r.DevicePath, allowedDevicePath)
	}
	return nil
}

func sudoOpenBlockDeviceHelperArgs(devicePath string) ([]string, error) {
	if err := validateDiskDevicePath(devicePath); err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if err := validateSecureHelperPath(exe); err != nil {
		return nil, err
	}
	return []string{exe, SudoOpenBlockDeviceCommand, devicePath}, nil
}

func validateSudoersUserName(userName string) error {
	if userName == "" {
		return errors.New("sudoers user name must not be empty")
	}
	if strings.ContainsAny(userName, " \t\r\n,:=\\") {
		return fmt.Errorf("sudoers user name %q contains unsupported characters", userName)
	}
	if strings.HasPrefix(userName, "%") || strings.HasPrefix(userName, "#") {
		return fmt.Errorf("sudoers user name %q must name a user, not a group or uid", userName)
	}
	return nil
}

func validateDiskDeviceNode(devicePath string) error {
	if err := validateDiskDevicePath(devicePath); err != nil {
		return err
	}
	fi, err := os.Lstat(devicePath)
	if err != nil {
		return fmt.Errorf("failed to lstat %q: %w", devicePath, err)
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%q must not be a symlink", devicePath)
	}
	return validateDiskDeviceMode(devicePath, fi.Mode())
}

func validateDiskDevicePath(devicePath string) error {
	if devicePath == "" {
		return errors.New("devicePath must not be empty")
	}
	if !filepath.IsAbs(devicePath) {
		return fmt.Errorf("devicePath %q must be an absolute path", devicePath)
	}
	if filepath.Clean(devicePath) != devicePath {
		return fmt.Errorf("devicePath %q must be normalized", devicePath)
	}
	if !macOSDiskDevicePathRE.MatchString(devicePath) {
		return fmt.Errorf("devicePath %q must be a macOS disk device path like /dev/disk4 or /dev/rdisk4s1", devicePath)
	}
	return nil
}

func validateDiskDeviceFile(devicePath string, file *os.File) error {
	fi, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat %q: %w", file.Name(), err)
	}
	return validateDiskDeviceMode(devicePath, fi.Mode())
}

func validateDiskDeviceMode(devicePath string, mode os.FileMode) error {
	if err := validateDiskDevicePath(devicePath); err != nil {
		return err
	}
	if mode&os.ModeDevice == 0 {
		return fmt.Errorf("%q is not a device node", devicePath)
	}
	isRaw := strings.HasPrefix(filepath.Base(devicePath), "rdisk")
	isChar := mode&os.ModeCharDevice != 0
	switch {
	case isRaw && !isChar:
		return fmt.Errorf("%q must be a raw character disk device", devicePath)
	case !isRaw && isChar:
		return fmt.Errorf("%q must be a block disk device", devicePath)
	}
	return nil
}

func sudoOriginalUID() (uint32, error) {
	sudoUID := os.Getenv("SUDO_UID")
	if sudoUID == "" {
		return 0, errors.New("SUDO_UID is not set; privileged helper must be launched by sudo")
	}
	uid, err := strconv.ParseUint(sudoUID, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid SUDO_UID %q: %w", sudoUID, err)
	}
	return uint32(uid), nil
}

func validatePrivateSocketPath(socketPath string, ownerUID uint32) error {
	if socketPath == "" {
		return errors.New("socketPath must not be empty")
	}
	if !filepath.IsAbs(socketPath) {
		return fmt.Errorf("socketPath %q must be an absolute path", socketPath)
	}
	if filepath.Clean(socketPath) != socketPath {
		return fmt.Errorf("socketPath %q must be normalized", socketPath)
	}
	if len(socketPath) >= osutil.UnixPathMax {
		return fmt.Errorf("socketPath %q must be less than UNIX_PATH_MAX=%d characters, but is %d", socketPath, osutil.UnixPathMax, len(socketPath))
	}
	parent := filepath.Dir(socketPath)
	fi, err := os.Lstat(parent)
	if err != nil {
		return fmt.Errorf("failed to lstat socket directory %q: %w", parent, err)
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("socket directory %q must not be a symlink", parent)
	}
	if !fi.Mode().IsDir() {
		return fmt.Errorf("socket directory %q is not a directory", parent)
	}
	stat, ok := osutil.SysStat(fi)
	if !ok {
		return fmt.Errorf("could not retrieve stat buffer for socket directory %q", parent)
	}
	if stat.Uid != ownerUID {
		return fmt.Errorf("socket directory %q is not owned by sudo user uid %d, but by uid %d", parent, ownerUID, stat.Uid)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("socket directory %q must not be accessible by group or other users", parent)
	}
	return nil
}

func removeStaleSocket(socketPath string) error {
	fi, err := os.Lstat(socketPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("refusing to remove socket path %q because it is a symlink", socketPath)
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to remove socket path %q because it is not a Unix socket", socketPath)
	}
	return os.Remove(socketPath)
}

func openDiskDeviceNoFollow(devicePath string) (*os.File, error) {
	openFlags := unix.O_RDWR | unix.O_CLOEXEC | unix.O_NOFOLLOW
	devFD, err := unix.Open(devicePath, openFlags, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(devFD), filepath.Base(devicePath)), nil
}

func validateSecureHelperPath(path string) error {
	root, err := osutil.LookupUser("root")
	if err != nil {
		return err
	}
	return validateSecurePath(path, root.Uid, root.Gid)
}

func validateSecurePath(path string, rootUID, rootGID uint32) error {
	if path == "" {
		return errors.New("path must not be empty")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path %q is not an absolute path", path)
	}
	if strings.ContainsAny(path, " \t\r\n") {
		return fmt.Errorf("path %q contains whitespace", path)
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if err := validateSecurePathElement(path, fi, rootUID, rootGID); err != nil {
		return err
	}
	if path != "/" {
		return validateSecurePath(filepath.Dir(path), rootUID, rootGID)
	}
	return nil
}

func validateSecurePathElement(path string, fi os.FileInfo, rootUID, rootGID uint32) error {
	file := "file"
	if fi.Mode().IsDir() {
		file = "dir"
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%s %q is a symlink", file, path)
	}
	stat, ok := osutil.SysStat(fi)
	if !ok {
		return fmt.Errorf("could not retrieve stat buffer for %q", path)
	}
	if stat.Uid != rootUID {
		return fmt.Errorf(`%s %q is not owned by root uid %d, but by uid %d`, file, path, rootUID, stat.Uid)
	}
	if fi.Mode()&0o20 != 0 && stat.Gid != rootGID {
		return fmt.Errorf(`%s %q is group-writable and group is not root gid %d, but is gid %d`, file, path, rootGID, stat.Gid)
	}
	if fi.Mode()&0o02 != 0 {
		return fmt.Errorf("%s %q is world-writable", file, path)
	}
	return nil
}

func generateNonce() (string, error) {
	var b [blockDeviceNonceHexLen / 2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func unixPeerUID(conn *net.UnixConn) (uint32, error) {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return 0, err
	}
	var uid uint32
	var sockErr error
	if err := rawConn.Control(func(fd uintptr) {
		var cred *unix.Xucred
		cred, sockErr = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if sockErr == nil {
			uid = cred.Uid
		}
	}); err != nil {
		return 0, err
	}
	if sockErr != nil {
		return 0, sockErr
	}
	return uid, nil
}

func validateUnixPeerUID(conn *net.UnixConn, expectedUID uint32) error {
	uid, err := unixPeerUID(conn)
	if err != nil {
		return fmt.Errorf("failed to get Unix socket peer credentials: %w", err)
	}
	if uid != expectedUID {
		return fmt.Errorf("refusing file descriptor from Unix socket peer uid %d; expected uid %d", uid, expectedUID)
	}
	return nil
}

// Open creates a private Unix socket, asks the privileged helper to open the
// host device and connect back to that socket, then returns a duplicated
// descriptor that the caller can retain for as long as needed.
func Open(ctx context.Context, devicePath, socketPath string) (*os.File, error) {
	if err := validateDiskDevicePath(devicePath); err != nil {
		return nil, err
	}
	helperArgs, err := sudoOpenBlockDeviceHelperArgs(devicePath)
	if err != nil {
		return nil, err
	}
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	currentUID64, err := strconv.ParseUint(currentUser.Uid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid current user uid %q: %w", currentUser.Uid, err)
	}
	if err := validatePrivateSocketPath(socketPath, uint32(currentUID64)); err != nil {
		return nil, err
	}
	root, err := osutil.LookupUser("root")
	if err != nil {
		return nil, err
	}
	if err := removeStaleSocket(socketPath); err != nil {
		return nil, err
	}

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %q: %w", socketPath, err)
	}
	listener.SetUnlinkOnClose(true)
	defer func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()

	req := sudoOpenBlockDeviceRequest{
		DevicePath: devicePath,
		SocketPath: socketPath,
	}
	req.Nonce, err = generateNonce()
	if err != nil {
		return nil, err
	}
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(req); err != nil {
		return nil, err
	}

	fdCh := make(chan receivedFD, 1)
	go func() {
		for {
			conn, err := listener.AcceptUnix()
			if err != nil {
				fdCh <- receivedFD{err: err}
				return
			}
			if err := validateUnixPeerUID(conn, root.Uid); err != nil {
				logrus.WithError(err).Warn("Ignoring unauthenticated block-device helper connection")
				_ = conn.Close()
				continue
			}
			nonce := make([]byte, len(req.Nonce))
			_ = conn.SetReadDeadline(time.Now().Add(helperReadTimeout))
			if _, err := io.ReadFull(conn, nonce); err != nil {
				_ = conn.Close()
				fdCh <- receivedFD{err: fmt.Errorf("failed to read authentication nonce: %w", err)}
				return
			}
			if !bytes.Equal(nonce, []byte(req.Nonce)) {
				_ = conn.Close()
				fdCh <- receivedFD{err: errors.New("refusing file descriptor from helper with invalid authentication nonce")}
				return
			}

			files, err := fd.Get(conn, 1, []string{filepath.Base(devicePath)})
			_ = conn.Close()
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
			if err := validateDiskDeviceFile(devicePath, files[0]); err != nil {
				_ = files[0].Close()
				fdCh <- receivedFD{err: err}
				return
			}
			fdCh <- receivedFD{file: files[0]}
			return
		}
	}()

	var stdout, stderr bytes.Buffer
	cmd := sudoers.NewCommand(ctx, "root", "wheel", &stdin, &stdout, &stderr, "", helperArgs...)
	logrus.Debugf("Opening block device %q via sudo helper: %v", devicePath, cmd.Args)
	if err := cmd.Run(); err != nil {
		_ = listener.Close()
		// After sudo has failed, do not wait for the helper response: the helper
		// may never connect back. Only close an fd that has already arrived.
		if receivedFile := drainReceivedFD(fdCh); receivedFile != nil {
			_ = receivedFile.Close()
		}
		return nil, fmt.Errorf("failed to run %v: stdout=%q, stderr=%q: %w. Hint: run `%s sudoers --block-device=%s` and install the generated file", cmd.Args, stdout.String(), stderr.String(), err, os.Args[0], devicePath)
	}

	receivedFile, err := waitForReceivedFD(ctx, fdCh)
	if err != nil {
		if receivedFile := drainReceivedFD(fdCh); receivedFile != nil {
			_ = receivedFile.Close()
		}
		return nil, fmt.Errorf("failed to receive file descriptor for %q: %w", devicePath, err)
	}
	defer receivedFile.Close()

	dupFile, err := duplicateFileCloseOnExec(receivedFile, filepath.Base(devicePath))
	if err != nil {
		return nil, fmt.Errorf("failed to duplicate file descriptor for %q: %w", devicePath, err)
	}
	return dupFile, nil
}

// waitForReceivedFD ties fd handoff to the caller context so a helper crash,
// socket failure, or canceled VM start cannot leave limactl blocked forever.
func waitForReceivedFD(ctx context.Context, fdCh <-chan receivedFD) (*os.File, error) {
	select {
	case received := <-fdCh:
		if received.err != nil {
			return nil, received.err
		}
		return received.file, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// drainReceivedFD is a non-blocking cleanup path for command failures.
func drainReceivedFD(fdCh <-chan receivedFD) *os.File {
	select {
	case received := <-fdCh:
		return received.file
	default:
		return nil
	}
}

// duplicateFileCloseOnExec returns a caller-owned duplicate without allowing
// the root-opened descriptor to leak into future exec'd child processes.
func duplicateFileCloseOnExec(file *os.File, name string) (*os.File, error) {
	dupFD, err := unix.FcntlInt(file.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(dupFD), name), nil
}
