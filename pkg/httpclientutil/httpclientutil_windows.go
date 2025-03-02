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

package httpclientutil

import (
	"context"
	"net"
	"net/http"
	"os"
)

// NewHTTPClientWithSocketPath creates a client.
// socketPath is a path to the UNIX socket, without unix:// prefix.
func NewHTTPClientWithSocketPath(socketPath string) (*http.Client, error) {
	// Use Lstat on windows, see: https://github.com/adrg/xdg/pull/14
	if _, err := os.Lstat(socketPath); err != nil {
		return nil, err
	}
	hc := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		},
	}
	return hc, nil
}
