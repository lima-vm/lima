// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sshutil

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-semver/semver"
	sshocker "github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/mattn/go-shellwords"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/cpu"

	"github.com/lima-vm/lima/v2/pkg/ioutilx"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/lockutil"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

// Environment variable that allows configuring the command (alias) to execute
// in place of the 'ssh' executable.
const EnvShellSSH = "SSH"

type SSHExe struct {
	Exe  string
	Args []string
}

func NewSSHExe() (SSHExe, error) {
	var sshExe SSHExe

	if sshShell := os.Getenv(EnvShellSSH); sshShell != "" {
		sshShellFields, err := shellwords.Parse(sshShell)
		switch {
		case err != nil:
			logrus.WithError(err).Warnf("Failed to split %s variable into shell tokens. "+
				"Falling back to 'ssh' command", EnvShellSSH)
		case len(sshShellFields) > 0:
			sshExe.Exe = sshShellFields[0]
			if len(sshShellFields) > 1 {
				sshExe.Args = sshShellFields[1:]
			}
			return sshExe, nil
		}
	}

	executable, err := exec.LookPath("ssh")
	if err != nil {
		return SSHExe{}, err
	}
	sshExe.Exe = executable

	return sshExe, nil
}

type PubKey struct {
	Filename string
	Content  string
}

func readPublicKey(f string) (PubKey, error) {
	entry := PubKey{
		Filename: f,
	}
	content, err := os.ReadFile(f)
	if err == nil {
		entry.Content = strings.TrimSpace(string(content))
	} else {
		err = fmt.Errorf("failed to read ssh public key %q: %w", f, err)
	}
	return entry, err
}

// DefaultPubKeys returns the public key from $LIMA_HOME/_config/user.pub.
// The key will be created if it does not yet exist.
//
// When loadDotSSH is true, ~/.ssh/*.pub will be appended to make the VM accessible without specifying
// an identity explicitly.
func DefaultPubKeys(ctx context.Context, loadDotSSH bool) ([]PubKey, error) {
	// Read $LIMA_HOME/_config/user.pub
	configDir, err := dirnames.LimaConfigDir()
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(filepath.Join(configDir, filenames.UserPrivateKey))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err := os.MkdirAll(configDir, 0o700); err != nil {
			return nil, fmt.Errorf("could not create %q directory: %w", configDir, err)
		}
		if err := lockutil.WithDirLock(configDir, func() error {
			// no passphrase, no user@host comment
			privPath := filepath.Join(configDir, filenames.UserPrivateKey)
			if runtime.GOOS == "windows" {
				privPath, err = ioutilx.WindowsSubsystemPath(ctx, privPath)
				if err != nil {
					return err
				}
			}
			keygenCmd := exec.CommandContext(ctx, "ssh-keygen", "-t", "ed25519", "-q", "-N", "",
				"-C", "lima", "-f", privPath)
			logrus.Debugf("executing %v", keygenCmd.Args)
			if out, err := keygenCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to run %v: %q: %w", keygenCmd.Args, string(out), err)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	entry, err := readPublicKey(filepath.Join(configDir, filenames.UserPublicKey))
	if err != nil {
		return nil, err
	}
	res := []PubKey{entry}

	if !loadDotSSH {
		return res, nil
	}

	// Append all of ~/.ssh/*.pub
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	files, err := filepath.Glob(filepath.Join(homeDir, ".ssh/*.pub"))
	if err != nil {
		panic(err) // Only possible error is ErrBadPattern, so this should be unreachable.
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".pub") {
			panic(fmt.Errorf("unexpected ssh public key filename %q", f))
		}
		entry, err := readPublicKey(f)
		if err == nil {
			if !detectValidPublicKey(entry.Content) {
				logrus.Warnf("public key %q doesn't seem to be in ssh format", entry.Filename)
			} else {
				res = append(res, entry)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return res, nil
}

type openSSHInfo struct {
	// Version is set to the version of OpenSSH, or semver.New("0.0.0") if the version cannot be determined.
	Version semver.Version

	// Some distributions omit this feature by default, for example, Alpine, NixOS.
	GSSAPISupported bool
}

var sshInfo struct {
	sync.Once
	// aesAccelerated is set to true when AES acceleration is available.
	// Available on almost all modern Intel/AMD processors.
	aesAccelerated bool

	// OpenSSH executable information for the version and supported options.
	openSSH openSSHInfo
}

// CommonOpts returns ssh option key-value pairs like {"IdentityFile=/path/to/id_foo"}.
// The result may contain different values with the same key.
//
// The result always contains the IdentityFile option.
// The result never contains the Port option.
func CommonOpts(ctx context.Context, sshExe SSHExe, useDotSSH bool) ([]string, error) {
	configDir, err := dirnames.LimaConfigDir()
	if err != nil {
		return nil, err
	}
	privateKeyPath := filepath.Join(configDir, filenames.UserPrivateKey)
	_, err = os.Stat(privateKeyPath)
	if err != nil {
		return nil, err
	}
	var opts []string
	idf, err := identityFileEntry(ctx, privateKeyPath)
	if err != nil {
		return nil, err
	}
	opts = []string{idf}

	// Append all private keys corresponding to ~/.ssh/*.pub to keep old instances working
	// that had been created before lima started using an internal identity.
	if useDotSSH {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		files, err := filepath.Glob(filepath.Join(homeDir, ".ssh/*.pub"))
		if err != nil {
			panic(err) // Only possible error is ErrBadPattern, so this should be unreachable.
		}
		for _, f := range files {
			if !strings.HasSuffix(f, ".pub") {
				panic(fmt.Errorf("unexpected ssh public key filename %q", f))
			}
			privateKeyPath := strings.TrimSuffix(f, ".pub")
			_, err = os.Stat(privateKeyPath)
			if errors.Is(err, fs.ErrNotExist) {
				// Skip .pub files without a matching private key. This is reasonably common,
				// due to major projects like Vault recommending the ${name}-cert.pub format
				// for SSH certificate files.
				//
				// e.g. https://www.vaultproject.io/docs/secrets/ssh/signed-ssh-certificates
				continue
			}
			if err != nil {
				// Fail on permission-related and other path errors
				return nil, err
			}
			idf, err = identityFileEntry(ctx, privateKeyPath)
			if err != nil {
				return nil, err
			}
			opts = append(opts, idf)
		}
	}

	opts = append(opts,
		"StrictHostKeyChecking=no",
		"UserKnownHostsFile=/dev/null",
		"NoHostAuthenticationForLocalhost=yes",
		"PreferredAuthentications=publickey",
		"Compression=no",
		"BatchMode=yes",
		"IdentitiesOnly=yes",
	)

	sshInfo.Do(func() {
		sshInfo.aesAccelerated = detectAESAcceleration()
		sshInfo.openSSH = detectOpenSSHInfo(ctx, sshExe)
	})

	if sshInfo.openSSH.GSSAPISupported {
		opts = append(opts, "GSSAPIAuthentication=no")
	}

	// Only OpenSSH version 8.1 and later support adding ciphers to the front of the default set
	if !sshInfo.openSSH.Version.LessThan(*semver.New("8.1.0")) {
		// By default, `ssh` choose chacha20-poly1305@openssh.com, even when AES accelerator is available.
		// (OpenSSH_8.1p1, macOS 11.6, MacBookPro 2020, Core i7-1068NG7)
		//
		// We prioritize AES algorithms when AES accelerator is available.
		if sshInfo.aesAccelerated {
			logrus.Debugf("AES accelerator seems available, prioritizing aes128-gcm@openssh.com and aes256-gcm@openssh.com")
			if runtime.GOOS == "windows" {
				opts = append(opts, "Ciphers=^aes128-gcm@openssh.com,aes256-gcm@openssh.com")
			} else {
				opts = append(opts, "Ciphers=\"^aes128-gcm@openssh.com,aes256-gcm@openssh.com\"")
			}
		} else {
			logrus.Debugf("AES accelerator does not seem available, prioritizing chacha20-poly1305@openssh.com")
			if runtime.GOOS == "windows" {
				opts = append(opts, "Ciphers=^chacha20-poly1305@openssh.com")
			} else {
				opts = append(opts, "Ciphers=\"^chacha20-poly1305@openssh.com\"")
			}
		}
	}
	return opts, nil
}

func identityFileEntry(ctx context.Context, privateKeyPath string) (string, error) {
	if runtime.GOOS == "windows" {
		privateKeyPath, err := ioutilx.WindowsSubsystemPath(ctx, privateKeyPath)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`IdentityFile='%s'`, privateKeyPath), nil
	}
	return fmt.Sprintf(`IdentityFile="%s"`, privateKeyPath), nil
}

// DisableControlMasterOptsFromSSHArgs returns ssh args that disable ControlMaster, ControlPath, and ControlPersist.
func DisableControlMasterOptsFromSSHArgs(sshArgs []string) []string {
	argsForOverridingConfigFile := []string{
		"-o", "ControlMaster=no",
		"-o", "ControlPath=none",
		"-o", "ControlPersist=no",
	}
	return slices.Concat(argsForOverridingConfigFile, removeOptsFromSSHArgs(sshArgs, "ControlMaster", "ControlPath", "ControlPersist"))
}

func removeOptsFromSSHArgs(sshArgs []string, removeOpts ...string) []string {
	res := make([]string, 0, len(sshArgs))
	isOpt := false
	for _, arg := range sshArgs {
		if isOpt {
			isOpt = false
			if !slices.ContainsFunc(removeOpts, func(opt string) bool {
				return strings.HasPrefix(arg, opt)
			}) {
				res = append(res, "-o", arg)
			}
		} else if arg == "-o" {
			isOpt = true
		} else {
			res = append(res, arg)
		}
	}
	return res
}

// IsControlMasterExisting returns true if the control socket file exists.
func IsControlMasterExisting(instDir string) bool {
	controlSock := filepath.Join(instDir, filenames.SSHSock)
	_, err := os.Stat(controlSock)
	return err == nil
}

// SSHOpts adds the following options to CommonOptions: User, ControlMaster, ControlPath, ControlPersist.
func SSHOpts(ctx context.Context, sshExe SSHExe, instDir, username string, useDotSSH, forwardAgent, forwardX11, forwardX11Trusted bool) ([]string, error) {
	controlSock := filepath.Join(instDir, filenames.SSHSock)
	if len(controlSock) >= osutil.UnixPathMax {
		return nil, fmt.Errorf("socket path %q is too long: >= UNIX_PATH_MAX=%d", controlSock, osutil.UnixPathMax)
	}
	opts, err := CommonOpts(ctx, sshExe, useDotSSH)
	if err != nil {
		return nil, err
	}
	controlPath := fmt.Sprintf(`ControlPath="%s"`, controlSock)
	if runtime.GOOS == "windows" {
		controlSock, err = ioutilx.WindowsSubsystemPath(ctx, controlSock)
		if err != nil {
			return nil, err
		}
		controlPath = fmt.Sprintf(`ControlPath='%s'`, controlSock)
	}
	opts = append(opts,
		fmt.Sprintf("User=%s", username), // guest and host have the same username, but we should specify the username explicitly (#85)
		"ControlMaster=auto",
		controlPath,
		"ControlPersist=yes",
	)
	if forwardAgent {
		opts = append(opts, "ForwardAgent=yes")
	}
	if forwardX11 {
		opts = append(opts, "ForwardX11=yes")
	}
	if forwardX11Trusted {
		opts = append(opts, "ForwardX11Trusted=yes")
	}
	return opts, nil
}

// SSHArgsFromOpts returns ssh args from opts.
// The result always contains {"-F", "/dev/null} in addition to {"-o", "KEY=VALUE", ...}.
func SSHArgsFromOpts(opts []string) []string {
	args := []string{"-F", "/dev/null"}
	for _, o := range opts {
		args = append(args, "-o", o)
	}
	return args
}

// SSHOptsRemovingControlPath removes ControlMaster, ControlPath, and ControlPersist options from SSH options.
func SSHOptsRemovingControlPath(opts []string) []string {
	// Create a copy of opts to avoid modifying the original slice, since slices.DeleteFunc modifies the slice in place.
	copiedOpts := slices.Clone(opts)
	return slices.DeleteFunc(copiedOpts, func(s string) bool {
		return strings.HasPrefix(s, "ControlMaster") || strings.HasPrefix(s, "ControlPath") || strings.HasPrefix(s, "ControlPersist")
	})
}

func ParseOpenSSHVersion(version []byte) *semver.Version {
	regex := regexp.MustCompile(`(?m)^OpenSSH_(\d+\.\d+)(?:p(\d+))?\b`)
	matches := regex.FindSubmatch(version)
	if len(matches) == 3 {
		if len(matches[2]) == 0 {
			matches[2] = []byte("0")
		}
		return semver.New(fmt.Sprintf("%s.%s", matches[1], matches[2]))
	}
	return &semver.Version{}
}

func parseOpenSSHGSSAPISupported(version string) bool {
	return !strings.Contains(version, `Unsupported option "gssapiauthentication"`)
}

// sshExecutable beyond path also records size and mtime, in the case of ssh upgrades.
type sshExecutable struct {
	Path    string
	Size    int64
	ModTime time.Time
}

var (
	// openSSHInfos caches the parsed version and supported options of each ssh executable, if it is needed again.
	openSSHInfos   = map[sshExecutable]*openSSHInfo{}
	openSSHInfosRW sync.RWMutex
)

func detectOpenSSHInfo(ctx context.Context, sshExe SSHExe) openSSHInfo {
	var (
		info   openSSHInfo
		exe    sshExecutable
		stderr bytes.Buffer
	)
	// Note: For SSH wrappers like "kitten ssh", os.Stat will check the wrapper
	// executable (kitten) instead of the underlying ssh binary. This means
	// cache invalidation won't work properly - ssh upgrades won't be detected
	// since kitten's size/mtime won't change. This is probably acceptable.
	if st, err := os.Stat(sshExe.Exe); err == nil {
		exe = sshExecutable{Path: sshExe.Exe, Size: st.Size(), ModTime: st.ModTime()}
		openSSHInfosRW.RLock()
		info := openSSHInfos[exe]
		openSSHInfosRW.RUnlock()
		if info != nil {
			return *info
		}
	}
	sshArgs := append([]string{}, sshExe.Args...)
	// -V should be last
	sshArgs = append(sshArgs, "-o", "GSSAPIAuthentication=no", "-V")
	cmd := exec.CommandContext(ctx, sshExe.Exe, sshArgs...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logrus.Warnf("failed to run %v: stderr=%q", cmd.Args, stderr.String())
	} else {
		info = openSSHInfo{
			Version:         *ParseOpenSSHVersion(stderr.Bytes()),
			GSSAPISupported: parseOpenSSHGSSAPISupported(stderr.String()),
		}
		logrus.Debugf("OpenSSH version %s detected, is GSSAPI supported: %t", info.Version, info.GSSAPISupported)
		openSSHInfosRW.Lock()
		openSSHInfos[exe] = &info
		openSSHInfosRW.Unlock()
	}
	return info
}

func DetectOpenSSHVersion(ctx context.Context, sshExe SSHExe) semver.Version {
	return detectOpenSSHInfo(ctx, sshExe).Version
}

// detectValidPublicKey returns whether content represent a public key.
// OpenSSH public key format have the structure of '<algorithm> <key> <comment>'.
// By checking 'algorithm' with signature format identifier in 'key' part,
// this function may report false positive but provide better compatibility.
func detectValidPublicKey(content string) bool {
	if strings.ContainsRune(content, '\n') {
		return false
	}
	spaced := strings.SplitN(content, " ", 3)
	if len(spaced) < 2 {
		return false
	}
	algo, base64Key := spaced[0], spaced[1]
	decodedKey, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil || len(decodedKey) < 4 {
		return false
	}
	sigLength := binary.BigEndian.Uint32(decodedKey)
	if uint32(len(decodedKey)) < sigLength {
		return false
	}
	sigFormat := string(decodedKey[4 : 4+sigLength])
	return algo == sigFormat
}

func detectAESAcceleration() bool {
	if !cpu.Initialized {
		if runtime.GOOS == "linux" && runtime.GOARCH == "arm64" {
			// cpu.Initialized seems to always be false, even when the cpu.ARM64 struct is filled out
			// it is only being set by readARM64Registers, but not by readHWCAP or readLinuxProcCPUInfo
			return cpu.ARM64.HasAES
		}
		if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
			// golang.org/x/sys/cpu supports darwin/amd64, linux/amd64, and linux/arm64,
			// but apparently lacks support for darwin/arm64: https://github.com/golang/sys/blob/v0.5.0/cpu/cpu_arm64.go#L43-L60
			//
			// According to https://gist.github.com/voluntas/fd279c7b4e71f9950cfd4a5ab90b722b ,
			// aes-128-gcm is faster than chacha20-poly1305 on Apple M1.
			//
			// So we return `true` here.
			//
			// This workaround will not be needed when https://go-review.googlesource.com/c/sys/+/332729 is merged.
			logrus.Debug("Failed to detect CPU features. Assuming that AES acceleration is available on this Apple silicon.")
			return true
		}
		logrus.Warn("Failed to detect CPU features. Assuming that AES acceleration is not available.")
		return false
	}
	return cpu.ARM.HasAES || cpu.ARM64.HasAES || cpu.PPC64.IsPOWER8 || cpu.S390X.HasAES || cpu.X86.HasAES
}

// WaitSSHReady waits until the SSH server is ready to accept connections.
// The dialContext function is used to create a connection to the SSH server.
// The addr, user, parameter is used for ssh.ClientConn creation.
// The timeoutSeconds parameter specifies the maximum number of seconds to wait.
func WaitSSHReady(ctx context.Context, dialContext func(context.Context) (net.Conn, error), addr, user string, timeoutSeconds int) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Prepare signer
	signer, err := userPrivateKeySigner()
	if err != nil {
		return err
	}
	// Prepare ssh client config
	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCollector().checker(),
		Timeout:         10 * time.Second,
	}
	// Wait until the SSH server is available.
	for {
		conn, err := dialContext(ctx)
		if err == nil {
			sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
			if err == nil {
				sshClient := ssh.NewClient(sshConn, chans, reqs)
				return sshClient.Close()
			}
			conn.Close()
			if !isRetryableError(err) {
				return fmt.Errorf("failed to create ssh.Conn to %q: %w", addr, err)
			}
		}
		logrus.Debugf("Waiting for SSH port to accept connections on %s", addr)
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed to waiting for SSH port to become available on %s: %w", addr, ctx.Err())
		case <-time.After(1 * time.Second):
			continue
		}
	}
}

// errHostKeyMismatch is returned when the SSH host key does not match known hosts.
var errHostKeyMismatch = errors.New("ssh: host key mismatch")

func isRetryableError(err error) bool {
	// Port forwarder accepted the connection, but the destination is not ready yet.
	return osutil.IsConnectionResetError(err) ||
		// SSH server not ready yet (e.g. host key not generated on initial boot).
		strings.HasSuffix(err.Error(), "no supported methods remain") ||
		// Host key is not yet in known_hosts, but will be collected, so we can retry.
		errors.Is(err, errHostKeyMismatch)
}

// userPrivateKeySigner returns the user's private key signer.
// The public key is always installed in the VM.
func userPrivateKeySigner() (ssh.Signer, error) {
	configDir, err := dirnames.LimaConfigDir()
	if err != nil {
		return nil, err
	}
	privateKeyPath := filepath.Join(configDir, filenames.UserPrivateKey)
	key, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key %q: %w", privateKeyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key %q: %w", privateKeyPath, err)
	}
	return signer, nil
}

// hostKeyCollector is a singleton host key collector.
var hostKeyCollector = sync.OnceValue(func() *_hostKeyCollector {
	return &_hostKeyCollector{
		hostKeys: make(map[string]ssh.PublicKey),
	}
})

type _hostKeyCollector struct {
	hostKeys map[string]ssh.PublicKey
	mu       sync.Mutex
}

// checker returns a HostKeyCallback that either checks and collects the host key,
// or only checks the host key, depending on whether any host keys have been collected.
// It is expected to pass host key checks by retrying after the first collection.
// On second invocation, it will only check the host key.
func (h *_hostKeyCollector) checker() ssh.HostKeyCallback {
	if len(h.hostKeys) == 0 {
		return h.checkAndCollect
	}
	return h.checkOnly
}

// checkAndCollect is a HostKeyCallback that records the host key provided by the SSH server.
func (h *_hostKeyCollector) checkAndCollect(_ string, _ net.Addr, key ssh.PublicKey) error {
	marshaledKey := string(key.Marshal())
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.hostKeys[marshaledKey]; ok {
		return nil
	}
	h.hostKeys[marshaledKey] = key
	// If always returning nil here, GitHub Advanced Security may report "Use of insecure HostKeyCallback implementation".
	// So, we return an error here to make the SSH client report the host key mismatch.
	return errHostKeyMismatch
}

// check is a HostKeyCallback that checks whether the host key has been collected.
func (h *_hostKeyCollector) checkOnly(_ string, _ net.Addr, key ssh.PublicKey) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.hostKeys[string(key.Marshal())]; ok {
		return nil
	}
	// If always returning nil here, GitHub Advanced Security may report "Use of insecure HostKeyCallback implementation".
	// So, we return an error here to make the SSH client report the host key mismatch.
	return errHostKeyMismatch
}

// ExecuteScriptViaInProcessClient executes the given script on the remote host via in-process SSH client.
func ExecuteScriptViaInProcessClient(host string, port int, user, script, scriptName string) (stdout, stderr string, err error) {
	// Prepare signer
	signer, err := userPrivateKeySigner()
	if err != nil {
		return "", "", err
	}

	// Prepare ssh client config
	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCollector().checker(),
		Timeout:         10 * time.Second,
	}

	// Connect to SSH server
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	var dialer net.Dialer
	dialer.Timeout = sshConfig.Timeout
	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		return "", "", fmt.Errorf("failed to dial %q: %w", addr, err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		return "", "", fmt.Errorf("failed to create ssh.Conn to %q: %w", addr, err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	if err != nil {
		return "", "", fmt.Errorf("failed to create SSH client to %q: %w", addr, err)
	}
	defer client.Close()

	// Create session
	session, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("failed to create SSH session to %q: %w", addr, err)
	}
	defer session.Close()

	// Execute script
	interpreter, err := sshocker.ParseScriptInterpreter(script)
	if err != nil {
		return "", "", err
	}
	// Provide the script via stdin
	session.Stdin = strings.NewReader(strings.TrimPrefix(script, "#!"+interpreter+"\n"))
	// Capture stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf
	logrus.Debugf("executing ssh for script %q", scriptName)
	err = session.Run(interpreter)
	if err != nil {
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("failed to execute script %q: stdout=%q, stderr=%q: %w", scriptName, stdoutBuf.String(), stderrBuf.String(), err)
	}
	return stdoutBuf.String(), stderrBuf.String(), nil
}
