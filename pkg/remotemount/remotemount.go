package remotemount

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"
)

type RemoteMount struct {
	*ssh.SSHConfig
	LocalPath           string
	Host                string
	Port                int
	RemotePath          string
	Readonly            bool
	sshCmd              *exec.Cmd
	MountType           string
	MountTag            string
	MountAdditionalArgs []string
}

func (rm *RemoteMount) sshCommand(args ...string) ([]byte, error) {
	sshBinary := rm.SSHConfig.Binary()
	sshArgs := rm.SSHConfig.Args()
	if rm.Port != 0 {
		sshArgs = append(sshArgs, "-p", strconv.Itoa(rm.Port))
	}
	sshArgs = append(sshArgs, rm.Host, "--")
	sshArgs = append(sshArgs, args...)
	rm.sshCmd = exec.Command(sshBinary, sshArgs...)
	logrus.Debugf("executing ssh: %s %v", rm.sshCmd.Path, rm.sshCmd.Args)
	return rm.sshCmd.CombinedOutput()
}

func (rm *RemoteMount) Prepare() error {
	if !filepath.IsAbs(rm.RemotePath) {
		return fmt.Errorf("unexpected relative path: %q", rm.RemotePath)
	}
	out, err := rm.sshCommand("sudo", "mkdir", "-p", rm.RemotePath)
	if err != nil {
		return fmt.Errorf("failed to mkdir %q (remote): %q: %w", rm.RemotePath, string(out), err)
	}
	return nil
}

func (rm *RemoteMount) Start() error {
	if !filepath.IsAbs(rm.RemotePath) {
		return fmt.Errorf("unexpected relative path: %q", rm.RemotePath)
	}
	sshArgs := []string{"mountpoint", rm.RemotePath}
	out, err := rm.sshCommand(sshArgs...)
	if err == nil && strings.Contains(string(out), "is a mountpoint") { // already mounted
		return nil
	}
	sshArgs = []string{"sudo", "mount", "-t", rm.MountType, rm.MountTag, rm.RemotePath}
	if rm.Readonly {
		sshArgs = append(sshArgs, "-o", "ro")
	}
	sshArgs = append(sshArgs, rm.MountAdditionalArgs...)
	out, err = rm.sshCommand(sshArgs...)
	if err != nil {
		return fmt.Errorf("failed to mount %q (remote): %q: %w", rm.RemotePath, string(out), err)
	}
	return nil
}

func (rm *RemoteMount) Close() error {
	if !filepath.IsAbs(rm.RemotePath) {
		return fmt.Errorf("unexpected relative path: %q", rm.RemotePath)
	}
	sshArgs := []string{"sudo", "umount", rm.RemotePath}
	out, err := rm.sshCommand(sshArgs...)
	if err != nil {
		return fmt.Errorf("failed to umount %q (remote): %q: %w", rm.RemotePath, string(out), err)
	}
	return nil
}
