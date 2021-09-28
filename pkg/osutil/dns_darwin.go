package osutil

import (
	"fmt"
	"strings"

	"github.com/lima-vm/lima/pkg/sysprof"
)

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

func proxyURL(proxy string, port int) string {
	if !strings.Contains(proxy, "://") {
		proxy = "http://" + proxy
	}
	if port != 0 {
		proxy = fmt.Sprintf("%s:%d", proxy, port)
	}
	return proxy
}

func ProxySettings() (map[string]string, error) {
	nwData, err := sysprof.NetworkData()
	if err != nil {
		return nil, err
	}
	env := make(map[string]string)
	if len(nwData) > 0 {
		// In case "en0" is not found, use the proxies of the first interface
		proxies := nwData[0].Proxies
		for _, nw := range nwData {
			if nw.Interface == "en0" {
				proxies = nw.Proxies
				break
			}
		}
		// Proxies with a username are not going to work because the password is stored in a keychain.
		// If users are fine with exposing the username/password, they can set the proxy to
		// "http://username:password@proxyhost.com" in the system settings (or in lima.yaml).
		if proxies.FTPEnable == "yes" && proxies.FTPUser == "" {
			env["ftp_proxy"] = proxyURL(proxies.FTPProxy, proxies.FTPPort)
		}
		if proxies.HTTPEnable == "yes" && proxies.HTTPUser == "" {
			env["http_proxy"] = proxyURL(proxies.HTTPProxy, proxies.HTTPPort)
		}
		if proxies.HTTPSEnable == "yes" && proxies.HTTPSUser == "" {
			env["https_proxy"] = proxyURL(proxies.HTTPSProxy, proxies.HTTPSPort)
		}
		// Not setting up "no_proxy" variable; the values from the proxies.ExceptionList are
		// not understood by most applications checking "no_proxy". The default value would
		// be "*.local,169.254/16". Users can always specify env.no_proxy in lima.yaml.
	}
	return env, nil
}
