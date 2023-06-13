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
)

type Client struct {
	Directory string

	client   *http.Client
	delegate *gvproxyclient.Client
	base     string
}

func (c *Client) UnExposeSSH(sshPort int) error {
	return c.delegate.Unexpose(&types.UnexposeRequest{
		Local:    fmt.Sprintf("127.0.0.1:%d", sshPort),
		Protocol: "tcp",
	})
}

func (c *Client) ResolveAndForwardSSH(vmMacAddr string, sshPort int) error {
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-timeout:
			return errors.New("usernet unable to resolve IP for SSH forwarding")
		case <-ticker.C:
			leases, err := c.leases()
			if err != nil {
				return err
			}

			for ipAddr, leaseAddr := range leases {
				if vmMacAddr == leaseAddr {
					err = c.delegate.Expose(&types.ExposeRequest{
						Local:    fmt.Sprintf("127.0.0.1:%d", sshPort),
						Remote:   fmt.Sprintf("%s:22", ipAddr),
						Protocol: "tcp",
					})
					if err != nil {
						return err
					}
					return nil
				}
			}
		}
	}
}

func (c *Client) leases() (map[string]string, error) {
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

func NewClient(endpointSock string) *Client {
	return create(endpointSock, "http://lima")
}

func create(sock string, base string) *Client {
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
	}
}
