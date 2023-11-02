package usernet

import "net"

// incIP increments IP address by 1.
func incIP(ip net.IP) net.IP {
	in := ip
	if v4 := ip.To4(); v4 != nil {
		in = v4
	}
	res := make([]byte, len(in))
	copy(res, in)
	for j := len(res) - 1; j >= 0; j-- {
		res[j]++
		if res[j] > 0 {
			return res
		}
	}
	return res
}
