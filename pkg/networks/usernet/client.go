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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	gvproxyclient "github.com/containers/gvisor-tap-vsock/pkg/client"
	"github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/httpclientutil"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/networks/usernet/dnshosts"
)

type Client struct {
	Directory string

	client   *http.Client
	delegate *gvproxyclient.Client
	base     string
	subnet   net.IP
}

func (c *Client) ConfigureDriver(ctx context.Context, driver *driver.BaseDriver) error {
	macAddress := limayaml.MACAddress(driver.Instance.Dir)
	ipAddress, err := c.ResolveIPAddress(ctx, macAddress)
	if err != nil {
		return err
	}
	err = c.ResolveAndForwardSSH(ipAddress, driver.SSHLocalPort)
	if err != nil {
		return err
	}
	hosts := driver.Instance.Config.HostResolver.Hosts
	hosts[fmt.Sprintf("%s.internal", driver.Instance.Hostname)] = ipAddress
	err = c.AddDNSHosts(hosts)
	return err
}

func (c *Client) UnExposeSSH(sshPort int) error {
	return c.delegate.Unexpose(&types.UnexposeRequest{
		Local:    fmt.Sprintf("127.0.0.1:%d", sshPort),
		Protocol: "tcp",
	})
}

func (c *Client) AddDNSHosts(hosts map[string]string) error {
	hosts["host.lima.internal"] = GatewayIP(c.subnet)
	zones := dnshosts.ExtractZones(hosts)
	for _, zone := range zones {
		err := c.delegate.AddDNS(&zone)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) ResolveAndForwardSSH(ipAddr string, sshPort int) error {
	err := c.delegate.Expose(&types.ExposeRequest{
		Local:    fmt.Sprintf("127.0.0.1:%d", sshPort),
		Remote:   fmt.Sprintf("%s:22", ipAddr),
		Protocol: "tcp",
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ResolveIPAddress(ctx context.Context, vmMacAddr string) (string, error) {
	resolveIPAddressTimeout := 2 * time.Minute
	resolveIPAddressTimeoutEnv := os.Getenv("LIMA_USERNET_RESOLVE_IP_ADDRESS_TIMEOUT")
	if resolveIPAddressTimeoutEnv != "" {
		if parsedTimeout, err := strconv.Atoi(resolveIPAddressTimeoutEnv); err == nil {
			resolveIPAddressTimeout = time.Duration(parsedTimeout) * time.Minute
		}
	}
	ctx, cancel := context.WithTimeout(ctx, resolveIPAddressTimeout)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return "", errors.New("usernet unable to resolve IP for SSH forwarding")
		case <-ticker.C:
			leases, err := c.Leases(ctx)
			if err != nil {
				return "", err
			}

			for ipAddr, leaseAddr := range leases {
				if vmMacAddr == leaseAddr {
					return ipAddr, nil
				}
			}
		}
	}
}

func (c *Client) Leases(ctx context.Context) (map[string]string, error) {
	u := fmt.Sprintf("%s%s", c.base, "/services/dhcp/leases")
	res, err := httpclientutil.Get(ctx, c.client, u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	dec := json.NewDecoder(res.Body)
	var leases map[string]string
	if err := dec.Decode(&leases); err != nil {
		return nil, err
	}
	return leases, nil
}

func NewClientByName(nwName string) *Client {
	endpointSock, err := Sock(nwName, EndpointSock)
	if err != nil {
		return nil
	}
	subnet, err := Subnet(nwName)
	if err != nil {
		return nil
	}
	return NewClient(endpointSock, subnet)
}

func NewClient(endpointSock string, subnet net.IP) *Client {
	return create(endpointSock, subnet, "http://lima")
}

func create(sock string, subnet net.IP, base string) *Client {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", sock)
			},
		},
	}
	delegate := gvproxyclient.New(client, "http://lima")
	return &Client{
		client:   client,
		delegate: delegate,
		base:     base,
		subnet:   subnet,
	}
}
