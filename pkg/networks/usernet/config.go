// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usernet

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/apparentlymart/go-cidr/cidr"

	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/osutil"
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

// SockWithDirectory return a usernet socket based on dir, name and sockType.
func SockWithDirectory(dir, name string, sockType SockType) (string, error) {
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

// PIDFile returns a path for usernet PID file.
func PIDFile(name string) (string, error) {
	dir, err := dirnames.LimaNetworksDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name, fmt.Sprintf("usernet_%s.pid", name)), nil
}

// SubnetCIDR returns a subnet in form of net.IPNet for the given network name.
func SubnetCIDR(name string) (*net.IPNet, error) {
	cfg, err := networks.LoadConfig()
	if err != nil {
		return nil, err
	}
	err = cfg.Check(name)
	if err != nil {
		return nil, err
	}
	_, ipNet, err := netmaskToCidr(cfg.Networks[name].Gateway, cfg.Networks[name].NetMask)
	if err != nil {
		return nil, err
	}
	return ipNet, err
}

// Subnet returns a subnet net.IP for the given network name.
func Subnet(name string) (net.IP, error) {
	cfg, err := networks.LoadConfig()
	if err != nil {
		return nil, err
	}
	err = cfg.Check(name)
	if err != nil {
		return nil, err
	}
	_, ipNet, err := netmaskToCidr(cfg.Networks[name].Gateway, cfg.Networks[name].NetMask)
	if err != nil {
		return nil, err
	}
	return ipNet.IP, err
}

// GatewayIP returns the 2nd IP for the given subnet.
func GatewayIP(subnet net.IP) string {
	return cidr.Inc(cidr.Inc(subnet)).String()
}

// DNSIP returns the 3rd IP for the given subnet.
func DNSIP(subnet net.IP) string {
	return cidr.Inc(cidr.Inc(cidr.Inc(subnet))).String()
}

// Leases returns a leases file based on network name.
func Leases(name string) (string, error) {
	dir, err := dirnames.LimaNetworksDir()
	if err != nil {
		return "", err
	}
	sockPath := filepath.Join(filepath.Join(dir, name), "leases.json")
	if len(sockPath) >= osutil.UnixPathMax {
		return "", fmt.Errorf("usernet leases path %q too long: must be less than UNIX_PATH_MAX=%d characters, but is %d",
			sockPath, osutil.UnixPathMax, len(sockPath))
	}
	return sockPath, nil
}

func netmaskToCidr(baseIP, netMask net.IP) (net.IP, *net.IPNet, error) {
	size, _ := net.IPMask(netMask.To4()).Size()
	return net.ParseCIDR(fmt.Sprintf("%s/%d", baseIP.String(), size))
}
