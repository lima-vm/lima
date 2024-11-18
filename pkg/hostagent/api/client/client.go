package client

// Forked from https://github.com/rootless-containers/rootlesskit/blob/v0.14.2/pkg/api/client/client.go
// Apache License 2.0

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/lima-vm/lima/pkg/hostagent/api"
	"github.com/lima-vm/lima/pkg/httpclientutil"
)

type HostAgentClient interface {
	HTTPClient() *http.Client
	Info(context.Context) (*api.Info, error)
	DriverConfig(context.Context, interface{}) (interface{}, error)
}

// NewHostAgentClient creates a client.
// socketPath is a path to the UNIX socket, without unix:// prefix.
func NewHostAgentClient(socketPath string) (HostAgentClient, error) {
	hc, err := httpclientutil.NewHTTPClientWithSocketPath(socketPath)
	if err != nil {
		return nil, err
	}
	return NewHostAgentClientWithHTTPClient(hc), nil
}

func NewHostAgentClientWithHTTPClient(hc *http.Client) HostAgentClient {
	return &client{
		Client:    hc,
		version:   "v1",
		dummyHost: "lima-hostagent",
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

func (c *client) DriverConfig(ctx context.Context, config interface{}) (interface{}, error) {
	u := fmt.Sprintf("http://%s/%s/driver/config", c.dummyHost, c.version)
	method := "GET"
	var body io.Reader
	if config != nil {
		method = "PATCH"
		b, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		body = bytes.NewBuffer(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := httpclientutil.Successful(resp); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}
