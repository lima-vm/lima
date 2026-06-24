// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

// Forked from https://github.com/rootless-containers/rootlesskit/blob/v0.14.2/pkg/api/client/client.go
// Apache License 2.0

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lima-vm/lima/v2/pkg/hostagent/api"
	"github.com/lima-vm/lima/v2/pkg/httpclientutil"
)

type HostAgentClient interface {
	HTTPClient() *http.Client
	Info(context.Context) (*api.Info, error)
	MountAdd(ctx context.Context, req *api.MountRequest) (*api.Mount, error)
	MountRemove(ctx context.Context, mountPoint string) error
	MountList(ctx context.Context) ([]api.Mount, error)
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

func (c *client) mountsURL() string {
	return fmt.Sprintf("http://%s/%s/mounts", c.dummyHost, c.version)
}

func (c *client) MountList(ctx context.Context) ([]api.Mount, error) {
	resp, err := httpclientutil.Get(ctx, c.HTTPClient(), c.mountsURL())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var mounts []api.Mount
	if err := json.NewDecoder(resp.Body).Decode(&mounts); err != nil {
		return nil, err
	}
	return mounts, nil
}

func (c *client) MountAdd(ctx context.Context, req *api.MountRequest) (*api.Mount, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, err := httpclientutil.Post(ctx, c.HTTPClient(), c.mountsURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var m api.Mount
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (c *client) MountRemove(ctx context.Context, mountPoint string) error {
	body, err := json.Marshal(api.MountRequest{MountPoint: mountPoint})
	if err != nil {
		return err
	}
	resp, err := httpclientutil.Delete(ctx, c.HTTPClient(), c.mountsURL(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
