package usernet

import (
	"fmt"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store/dirnames"
)

type SockType = string

const (
	FDSock       = "fd"
	QEMUSock     = "qemu"
	EndpointSock = "ep"
)

// Sock returns a usernet socket based on name and sockType.
func Sock(name string, sockType SockType) (string, error) {
	dir, err := dirnames.LimaNetworksDir()
	if err != nil {
		return "", err
	}
	return SockWithDirectory(filepath.Join(dir, name), name, sockType)
}

// SockWithDirectory return a usernet socket based on dir, name and sockType
func SockWithDirectory(dir string, name string, sockType SockType) (string, error) {
	if name == "" {
		name = "default"
	}
	sockPath := filepath.Join(dir, fmt.Sprintf("%s_%s.sock", name, sockType))
	if len(sockPath) >= osutil.UnixPathMax {
		return "", fmt.Errorf("usernet socket path %q too long: must be less than UNIX_PATH_MAX=%d characters, but is %d",
			sockPath, osutil.UnixPathMax, len(sockPath))
	}
	return sockPath, nil
}

// PIDFile returns a path for usernet PID file
func PIDFile(name string) (string, error) {
	dir, err := dirnames.LimaNetworksDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name, fmt.Sprintf("usernet_%s.pid", name)), nil
}
