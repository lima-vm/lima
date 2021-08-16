package sshutil

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/lima-vm/lima/pkg/lockutil"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

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
		err = errors.Wrapf(err, "failed to read ssh public key %q", f)
	}
	return entry, err
}

// DefaultPubKeys returns the public key from $LIMA_HOME/_config/user.pub.
// The key will be created if it does not yet exist.
//
// When loadDotSSH is true, ~/.ssh/*.pub will be appended to make the VM accessible without specifying
// an identity explicitly.
func DefaultPubKeys(loadDotSSH bool) ([]PubKey, error) {
	// Read $LIMA_HOME/_config/user.pub
	configDir, err := store.LimaConfigDir()
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(filepath.Join(configDir, filenames.UserPrivateKey))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err := os.MkdirAll(configDir, 0700); err != nil {
			return nil, errors.Wrapf(err, "could not create %q directory", configDir)
		}
		if err := lockutil.WithDirLock(configDir, func() error {
			keygenCmd := exec.Command("ssh-keygen", "-t", "ed25519", "-q", "-N", "", "-f",
				filepath.Join(configDir, filenames.UserPrivateKey))
			logrus.Debugf("executing %v", keygenCmd.Args)
			if out, err := keygenCmd.CombinedOutput(); err != nil {
				return errors.Wrapf(err, "failed to run %v: %q", keygenCmd.Args, string(out))
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
			panic(errors.Errorf("unexpected ssh public key filename %q", f))
		}
		entry, err := readPublicKey(f)
		if err == nil {
			res = append(res, entry)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return res, nil
}

func RemoveKnownHostEntries(sshLocalPort int) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// `ssh-keygen -R` will return a non-0 status when ~/.ssh/known_hosts doesn't exist
	if _, err := os.Stat(filepath.Join(homeDir, ".ssh/known_hosts")); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	sshFixCmd := exec.Command("ssh-keygen",
		"-R", fmt.Sprintf("[127.0.0.1]:%d", sshLocalPort),
		"-R", fmt.Sprintf("[localhost]:%d", sshLocalPort),
	)
	if out, err := sshFixCmd.CombinedOutput(); err != nil {
		return errors.Wrapf(err, "failed to run %v: %q", sshFixCmd.Args, string(out))
	}
	return nil
}

func CommonArgs(useDotSSH bool) ([]string, error) {
	configDir, err := store.LimaConfigDir()
	if err != nil {
		return nil, err
	}
	privateKeyPath := filepath.Join(configDir, filenames.UserPrivateKey)
	_, err = os.Stat(privateKeyPath)
	if err != nil {
		return nil, err
	}
	args := []string{"-i", privateKeyPath}

	// Append all private keys corresponding to ~/.ssh/*.pub to keep old instances workin
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
				panic(errors.Errorf("unexpected ssh public key filename %q", f))
			}
			privateKeyPath := strings.TrimSuffix(f, ".pub")
			_, err = os.Stat(privateKeyPath)
			if err != nil {
				return nil, err
			}
			args = append(args, "-i", privateKeyPath)
		}
	}

	args = append(args,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "NoHostAuthenticationForLocalhost=yes",
		"-o", "GSSAPIAuthentication=no",
		"-o", "PreferredAuthentications=publickey",
		"-o", "Compression=no",
		"-o", "BatchMode=yes",
		"-o", "IdentitiesOnly=yes",
		"-F", "/dev/null",
	)
	return args, nil
}

func SSHArgs(instDir string, useDotSSH bool) ([]string, error) {
	controlSock := filepath.Join(instDir, filenames.SSHSock)
	if len(controlSock) >= osutil.UnixPathMax {
		return nil, errors.Errorf("socket path %q is too long: >= UNIX_PATH_MAX=%d", controlSock, osutil.UnixPathMax)
	}
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	args, err := CommonArgs(useDotSSH)
	if err != nil {
		return nil, err
	}
	args = append(args,
		"-l", u.Username, // guest and host have the same username, but we should specify the username explicitly (#85)
		"-o", "ControlMaster=auto",
		"-o", "ControlPath="+controlSock,
		"-o", "ControlPersist=5m",
	)
	return args, nil
}
