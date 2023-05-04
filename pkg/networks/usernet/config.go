package usernet

import (
	"fmt"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/store/dirnames"
)

type SockType = string

const (
	FDSock       = "fd"
	QEMUSock     = "qemu"
	EndpointSock = "endpoint"
)

// Sock returns a usernet socket based on name and sockType.
func Sock(name string, sockType SockType) (string, error) {
	dir, err := dirnames.LimaNetworksDir()
	if err != nil {
		return "", err
	}
	return SockWithDirectory(filepath.Join(dir, name), name, sockType), nil
}

// SockWithDirectory return a usernet socket based on dir, name and sockType
func SockWithDirectory(dir string, name string, sockType SockType) string {
	return filepath.Join(dir, fmt.Sprintf("usernet_%s_%s.sock", name, sockType))
}

// PIDFile returns a path for usernet PID file
func PIDFile(name string) (string, error) {
	dir, err := dirnames.LimaNetworksDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name, fmt.Sprintf("usernet_%s.pid", name)), nil
}
