package client

// Forked from https://github.com/rootless-containers/rootlesskit/blob/v0.14.2/pkg/api/client/client.go
// Apache License 2.0

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/lima-vm/lima/pkg/httpclientutil"
)

type GuestAgentClient interface {
	HTTPClient() *http.Client
	Info(context.Context) (*api.Info, error)
	Events(context.Context, func(api.Event)) error
}

type Proto = string

const (
	UNIX  Proto = "unix"
	VSOCK Proto = "vsock"
)

// NewGuestAgentClient creates a client.
// remote is a path to the UNIX socket, without unix:// prefix or a remote hostname/IP address.
func NewGuestAgentClient(remote string, proto Proto, instanceName string) (GuestAgentClient, error) {
	var hc *http.Client
	switch proto {
	case UNIX:
		hcSock, err := httpclientutil.NewHTTPClientWithSocketPath(remote)
		if err != nil {
			return nil, err
		}
		hc = hcSock
	case VSOCK:
		_, p, err := net.SplitHostPort(remote)
		if err != nil {
			return nil, err
		}
		port, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		hc, err = newVSockGuestAgentClient(port, instanceName)
		if err != nil {
			return nil, err
		}
	}

	return NewGuestAgentClientWithHTTPClient(hc), nil
}

func NewGuestAgentClientWithHTTPClient(hc *http.Client) GuestAgentClient {
	return &client{
		Client:    hc,
		version:   "v1",
		dummyHost: "lima-guestagent",
	}
}

type client struct {
	*http.Client
	// version is always "v1"
	// TODO(AkihiroSuda): negotiate the version
	version   string
	dummyHost string
}

func (c *client) HTTPClient() *http.Client {
	return c.Client
}

func (c *client) Info(ctx context.Context) (*api.Info, error) {
	u := fmt.Sprintf("http://%s/%s/info", c.dummyHost, c.version)
	resp, err := httpclientutil.Get(ctx, c.HTTPClient(), u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var info api.Info
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *client) Events(ctx context.Context, onEvent func(api.Event)) error {
	u := fmt.Sprintf("http://%s/%s/events", c.dummyHost, c.version)
	resp, err := httpclientutil.Get(ctx, c.HTTPClient(), u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	for {
		var ev api.Event
		if err := dec.Decode(&ev); err != nil {
			return err
		}
		onEvent(ev)
	}
}
