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

package client

// Forked from https://github.com/rootless-containers/rootlesskit/blob/v0.14.2/pkg/api/client/client.go
// Apache License 2.0

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lima-vm/lima/pkg/hostagent/api"
	"github.com/lima-vm/lima/pkg/httpclientutil"
)

type HostAgentClient interface {
	HTTPClient() *http.Client
	Info(context.Context) (*api.Info, error)
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
