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

package osutil

import (
	"fmt"
	"net"
	"strings"

	"github.com/lima-vm/lima/pkg/sysprof"
)

func DNSAddresses() ([]string, error) {
	nwData, err := sysprof.NetworkData()
	if err != nil {
		return nil, err
	}
	var addresses []string
	// Return DNS addresses from the first interface that has an IPv4 address.
	// The networks are in service order already.
	for _, nw := range nwData {
		if len(nw.IPv4.Addresses) > 0 {
			addresses = nw.DNS.ServerAddresses
			break
		}
	}
	return addresses, nil
}

func proxyURL(proxy string, port any) string {
	if strings.Contains(proxy, "://") {
		if portNumber, ok := port.(float64); ok && portNumber != 0 {
			proxy = fmt.Sprintf("%s:%.0f", proxy, portNumber)
		} else if portString, ok := port.(string); ok && portString != "" {
			proxy = fmt.Sprintf("%s:%s", proxy, portString)
		}
	} else {
		if portNumber, ok := port.(float64); ok && portNumber != 0 {
			proxy = net.JoinHostPort(proxy, fmt.Sprintf("%.0f", portNumber))
		} else if portString, ok := port.(string); ok && portString != "" {
			proxy = net.JoinHostPort(proxy, portString)
		}
		proxy = "http://" + proxy
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
		// Return proxy settings from the first interface that has an IPv4 address.
		// The networks are in service order already.
		var proxies sysprof.Proxies
		for _, nw := range nwData {
			if len(nw.IPv4.Addresses) > 0 {
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
