package metricutil

import (
	"errors"
	"net"
)

// getIP return default ip address on the guest instance.
// There would be several interfaces in a guest, This function return the default route to host.lima.internal.
func GetDefaultIP(ipVersion string) (string, error) {
	var dest string

	switch ipVersion {
	case "4":
		dest = "host.lima.internal:1"
	default:
		return "", errors.New("unsupported ipVersion: " + ipVersion)
	}

	conn, err := net.Dial("udp", dest)
	if err != nil {
		err = errors.New("cannot determine the default IP:")
		return "", err
	}
	defer conn.Close()

	ipAddress, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return "", err
	}

	return ipAddress, err
}
