// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

// Forked from https://github.com/rootless-containers/rootlesskit/blob/v0.14.2/pkg/api/client/client.go
// Apache License 2.0

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/hostagent/api"
	"github.com/lima-vm/lima/v2/pkg/httpclientutil"
)

type HostAgentClient interface {
	HTTPClient() *http.Client
	Info(context.Context) (*api.Info, error)
	GetCurrentMemory(context.Context) (int64, error)
	SetTargetMemory(context.Context, int64) error
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

func (c *client) GetCurrentMemory(ctx context.Context) (int64, error) {
	u := fmt.Sprintf("http://%s/%s/memory", c.dummyHost, c.version)
	resp, err := httpclientutil.Get(ctx, c.HTTPClient(), u)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	memory, err := strconv.ParseInt(string(body), 10, 64)
	if err != nil {
		return 0, err
	}
	return memory, nil
}

func (c *client) SetTargetMemory(ctx context.Context, memory int64) error {
	u := fmt.Sprintf("http://%s/%s/memory", c.dummyHost, c.version)
	body := strconv.FormatInt(memory, 10)
	b := strings.NewReader(body)
	resp, err := httpclientutil.Put(ctx, c.HTTPClient(), u, b)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
