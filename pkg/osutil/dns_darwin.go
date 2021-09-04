package osutil

import "github.com/lima-vm/lima/pkg/sysprof"

func DNSAddresses() ([]string, error) {
	nwData, err := sysprof.NetworkData()
	if err != nil {
		return nil, err
	}
	var addresses []string
	if len(nwData) > 0 {
		// Return DNS addresses from en0 interface
		for _, nw := range nwData {
			if nw.Interface == "en0" {
				addresses = nw.DNS.ServerAddresses
				break
			}
		}
		// In case "en0" is not found, use the addresses of the first interface
		if len(addresses) == 0 {
			addresses = nwData[0].DNS.ServerAddresses
		}
	}
	return addresses, nil
}
