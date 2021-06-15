package sshutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AkihiroSuda/lima/pkg/store/filenames"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type PubKey struct {
	Filename string
	Content  string
}

// DefaultPubKeys finds ssh public keys from ~/.ssh
func DefaultPubKeys() []PubKey {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logrus.Warn(err)
		return nil
	}
	files, err := filepath.Glob(filepath.Join(homeDir, ".ssh/*.pub"))
	if err != nil {
		logrus.Warn(err)
		return nil
	}
	var res []PubKey
	for _, f := range files {
		if !strings.HasSuffix(f, ".pub") {
			panic(errors.Errorf("unexpected ssh public key filename %q", f))
		}
		entry := PubKey{
			Filename: f,
		}
		if content, err := os.ReadFile(f); err == nil {
			entry.Content = strings.TrimSpace(string(content))
		} else {
			logrus.WithError(err).Warningf("failed to read ssh public key %q", f)
		}
		res = append(res, entry)
	}
	return res
}

func SSHArgs(instDir string) ([]string, error) {
	controlSock := filepath.Join(instDir, filenames.SSHSock)
	maxSockLen := 104
	if runtime.GOOS == "linux" {
		maxSockLen = 108
	}
	if len(controlSock) >= maxSockLen {
		// If the instDir was under `~/Library/Application Support/Lima`,
		// we could only have 54 chars for the user name and the inst name at maximum:
		// len("/Users/32charsXXXXXXXXXXXXXXXXXXXXXXXXX/Library/Application Support/Lima/22charsXXXXXXXXXXXXXXX/ssh.sock") = 104
		// So we do not use `~/Library/Application Support`.
		return nil, errors.Errorf("socket path %q is too long: > UNIX_PATH_MAX=%d", controlSock, maxSockLen)
	}
	args := []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + controlSock,
		"-o", "ControlPersist=5m",
		"-o", "StrictHostKeyChecking=no",
		"-o", "NoHostAuthenticationForLocalhost=yes",
		"-o", "GSSAPIAuthentication=no",
		"-o", "PreferredAuthentications=publickey",
		"-o", "Compression=no",
		"-o", "BatchMode=yes",
	}
	return args, nil
}
