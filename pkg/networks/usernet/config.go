/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package usernet

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/lima-vm/lima/pkg/networks"
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
