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
