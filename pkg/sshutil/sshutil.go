// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sshutil

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
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
	"github.com/mattn/go-shellwords"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/sys/cpu"

	"github.com/lima-vm/lima/v2/pkg/instance/hostname"
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
	userKnownHostsPath := filepath.Join(instDir, filenames.SSHKnownHosts)
	userKnownHosts := fmt.Sprintf(`UserKnownHostsFile="%s"`, userKnownHostsPath)
	if runtime.GOOS == "windows" {
		controlSock, err = ioutilx.WindowsSubsystemPath(ctx, controlSock)
		if err != nil {
			return nil, err
		}
		controlPath = fmt.Sprintf(`ControlPath='%s'`, controlSock)
		userKnownHostsPath, err = ioutilx.WindowsSubsystemPath(ctx, userKnownHostsPath)
		if err != nil {
			return nil, err
		}
		userKnownHosts = fmt.Sprintf(`UserKnownHostsFile='%s'`, userKnownHostsPath)
	}
	hostKeyAlias := fmt.Sprintf("HostKeyAlias=%s", hostname.FromInstName(filepath.Base(instDir)))
	opts = append(opts,
		fmt.Sprintf("User=%s", username), // guest and host have the same username, but we should specify the username explicitly (#85)
		"ControlMaster=auto",
		controlPath,
		"ControlPersist=yes",
		userKnownHosts,
		hostKeyAlias,
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
// The addr, user, privateKeyPath parameter is used for ssh.ClientConn creation.
// The timeoutSeconds parameter specifies the maximum number of seconds to wait.
func WaitSSHReady(ctx context.Context, dialContext func(context.Context) (net.Conn, error), addr, user, instanceName string, timeoutSeconds int) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Prepare signer
	signer, err := UserPrivateKey()
	if err != nil {
		return err
	}
	// Prepare HostKeyCallback
	hostKeyChecker, err := HostKeyCheckerWithKeysInKnownHosts(instanceName)
	if err != nil {
		return err
	}
	// Prepare ssh client config
	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyChecker,
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

func isRetryableError(err error) bool {
	// Port forwarder accepted the connection, but the destination is not ready yet.
	return osutil.IsConnectionResetError(err) ||
		// SSH server not ready yet (e.g. host key not generated on initial boot).
		strings.HasSuffix(err.Error(), "no supported methods remain")
}

// UserPrivateKey returns the user's private key signer.
// The public key is always installed in the VM.
func UserPrivateKey() (ssh.Signer, error) {
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

func HostKeyCheckerWithKeysInKnownHosts(instanceName string) (ssh.HostKeyCallback, error) {
	publicKeys, err := PublicKeysFromKnownHosts(instanceName)
	if err != nil {
		return nil, err
	}
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		keyBytes := key.Marshal()
		for _, pk := range publicKeys {
			if bytes.Equal(keyBytes, pk.Marshal()) {
				return nil
			}
		}
		return errors.New("ssh: host key mismatch")
	}, nil
}

// PublicKeysFromKnownHosts returns the public keys from the known_hosts file located in the instance directory.
func PublicKeysFromKnownHosts(instanceName string) ([]ssh.PublicKey, error) {
	// Load known_hosts from the instance directory
	instanceDir, err := dirnames.InstanceDir(instanceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance dir for instance %q: %w", instanceName, err)
	}
	knownHostsPath := filepath.Join(instanceDir, filenames.SSHKnownHosts)
	knownHostsBytes, err := os.ReadFile(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read known_hosts file at %s: %w", knownHostsPath, err)
	}
	var publicKeys []ssh.PublicKey
	rest := knownHostsBytes
	for len(rest) > 0 {
		var publicKey ssh.PublicKey
		publicKey, _, _, rest, err = ssh.ParseAuthorizedKey(rest)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key from known_hosts file %s: %w", knownHostsPath, err)
		}
		publicKeys = append(publicKeys, publicKey)
	}
	return publicKeys, nil
}

// GenerateSSHHostKeys generates an Ed25519 host key pair for the SSH server.
// The private key is returned in PEM format, and the public key.
func GenerateSSHHostKeys(instDir, hostname string) (map[string]string, error) {
	generators := map[string]func(io.Reader) (crypto.PrivateKey, error){
		"ecdsa": func(rand io.Reader) (crypto.PrivateKey, error) {
			return ecdsa.GenerateKey(elliptic.P256(), rand)
		},
		"ed25519": func(rand io.Reader) (crypto.PrivateKey, error) {
			_, priv, err := ed25519.GenerateKey(rand)
			return priv, err
		},
		"rsa": func(rand io.Reader) (crypto.PrivateKey, error) {
			return rsa.GenerateKey(rand, 3072)
		},
	}
	res := make(map[string]string, len(generators))
	var sshKnownHosts []byte
	for keyType, generator := range generators {
		priv, err := generator(rand.Reader)
		if err != nil {
			return nil, err
		}
		privPem, err := ssh.MarshalPrivateKey(priv, hostname)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal %s private key to PEM format: %w", keyType, err)
		}
		pub, err := ssh.NewPublicKey(priv.(crypto.Signer).Public())
		if err != nil {
			return nil, fmt.Errorf("failed to create ssh %s public key: %w", keyType, err)
		}
		res[keyType+"_private"] = string(pem.EncodeToMemory(privPem))
		res[keyType+"_public"] = string(ssh.MarshalAuthorizedKey(pub))
		sshKnownHosts = append(sshKnownHosts, knownhosts.Line([]string{hostname}, pub)...)
		sshKnownHosts = append(sshKnownHosts, '\n')
	}
	knownHostsPath := filepath.Join(instDir, filenames.SSHKnownHosts)
	if err := os.WriteFile(knownHostsPath, sshKnownHosts, 0o644); err != nil {
		return nil, fmt.Errorf("failed to write known_hosts file at %s: %w", knownHostsPath, err)
	}
	return res, nil
}
