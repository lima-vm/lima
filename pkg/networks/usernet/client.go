package usernet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	gvproxyclient "github.com/containers/gvisor-tap-vsock/pkg/client"
	"github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/lima-vm/lima/pkg/driver"
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

func (c *Client) ConfigureDriver(driver *driver.BaseDriver) error {
	macAddress := limayaml.MACAddress(driver.Instance.Dir)
	ipAddress, err := c.ResolveIPAddress(macAddress)
	if err != nil {
		return err
	}
	err = c.ResolveAndForwardSSH(ipAddress, driver.SSHLocalPort)
	if err != nil {
		return err
	}
	hosts := driver.Yaml.HostResolver.Hosts
	hosts[fmt.Sprintf("lima-%s.internal", driver.Instance.Name)] = ipAddress
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

func (c *Client) ResolveIPAddress(vmMacAddr string) (string, error) {
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-timeout:
			return "", errors.New("usernet unable to resolve IP for SSH forwarding")
		case <-ticker.C:
			leases, err := c.Leases()
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

func (c *Client) Leases() (map[string]string, error) {
	res, err := c.client.Get(fmt.Sprintf("%s%s", c.base, "/services/dhcp/leases"))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", res.StatusCode)
	}
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
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", sock)
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
