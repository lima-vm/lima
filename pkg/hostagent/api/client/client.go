// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

// Forked from https://github.com/rootless-containers/rootlesskit/blob/v0.14.2/pkg/api/client/client.go
// Apache License 2.0

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/lima-vm/lima/v2/pkg/hostagent/api"
	"github.com/lima-vm/lima/v2/pkg/httpclientutil"
)

type HostAgentClient interface {
	HTTPClient() *http.Client
	Info(context.Context) (*api.Info, error)
	// Screenshot captures the VM display. format is "png" or "bmp" (default "png").
	Screenshot(context.Context, string) ([]byte, error)
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

func (c *client) Screenshot(ctx context.Context, format string) ([]byte, error) {
	if format == "" {
		format = "png"
	}
	if format != "png" && format != "bmp" {
		return nil, fmt.Errorf("unsupported format %q: must be png or bmp", format)
	}
	params := url.Values{"format": {format}}
	u := fmt.Sprintf("http://%s/%s/screenshot?%s", c.dummyHost, c.version, params.Encode())
	resp, err := httpclientutil.Get(ctx, c.HTTPClient(), u)
	if err != nil {
		var httpErr *httpclientutil.HTTPStatusError
		if errors.As(err, &httpErr) {
			switch httpErr.StatusCode {
			case http.StatusNotFound:
				return nil, errors.New("hostagent does not support screenshot — restart the instance to pick up the latest hostagent binary")
			case http.StatusNotImplemented:
				// httpErr.Error() extracts ErrorJSON.Message from the body, which
				// includes the driver name set by the hostagent.
				return nil, httpErr
			case http.StatusUnprocessableEntity:
				return nil, errors.New("VM has no display configured — set video.display (e.g., \"default\") in the instance config")
			}
		}
		return nil, err
	}
	defer resp.Body.Close()
	const maxScreenshotSize = 32 << 20 // 32 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxScreenshotSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxScreenshotSize {
		return nil, fmt.Errorf("screenshot response exceeds %d MB limit", maxScreenshotSize>>20)
	}
	return data, nil
}
