// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

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

	"github.com/lima-vm/lima/v2/pkg/httpclientutil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/networks/usernet/dnshosts"
)

type Client struct {
	Directory string

	client   *http.Client
	delegate *gvproxyclient.Client
	base     string
	subnet   net.IP
}

func (c *Client) ConfigureDriver(ctx context.Context, inst *limatype.Instance, sshLocalPort int) error {
	macAddress := limayaml.MACAddress(inst.Dir)
	ipAddress, err := c.ResolveIPAddress(ctx, macAddress)
	if err != nil {
		return err
	}
	if sshLocalPort != 0 {
		err = c.ResolveAndForwardSSH(ipAddress, sshLocalPort)
		if err != nil {
			return err
		}
	}
	hosts := inst.Config.HostResolver.Hosts
	if hosts == nil {
		hosts = make(map[string]string)
	}
	hosts[fmt.Sprintf("%s.internal", inst.Hostname)] = ipAddress
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

// WaitOpeningSSHPort Wait until the guest ssh server is available.
func (c *Client) WaitOpeningSSHPort(ctx context.Context, inst *limatype.Instance) error {
	// This timeout is based on the maximum wait time for the first essential requirement.
	timeoutSeconds := 600
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	macAddress := limayaml.MACAddress(inst.Dir)
	ipAddr, err := c.ResolveIPAddress(ctx, macAddress)
	if err != nil {
		return err
	}
	// -1 avoids both sides timing out simultaneously.
	u := fmt.Sprintf("%s/extension/wait_port?ip=%s&port=22&timeout=%d", c.base, ipAddr, timeoutSeconds-1)
	res, err := httpclientutil.Get(ctx, c.client, u)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return errors.New("failed to wait for SSH port")
	}
	return nil
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
