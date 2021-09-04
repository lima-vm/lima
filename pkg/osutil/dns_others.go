//go:build !darwin
// +build !darwin

package osutil

func DNSAddresses() ([]string, error) {
	// TODO: parse /etc/resolv.conf?
	return []string{}, nil
}
