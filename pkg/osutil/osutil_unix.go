//go:build !windows

package osutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func Dup2(oldfd, newfd int) (err error) {
	return unix.Dup2(oldfd, newfd)
}

func SignalName(sig os.Signal) string {
	return unix.SignalName(sig.(syscall.Signal))
}

func Sysctl(name string) (string, error) {
	var stderrBuf bytes.Buffer
	cmd := exec.Command("sysctl", "-n", name)
	cmd.Stderr = &stderrBuf
	stdout, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run %v: %w (stdout=%q, stderr=%q)", cmd.Args, err,
			string(stdout), stderrBuf.String())
	}
	return strings.TrimSuffix(string(stdout), "\n"), nil
}
