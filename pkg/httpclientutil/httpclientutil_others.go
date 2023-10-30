//go:build !windows

package httpclientutil

import (
	"context"
	"net"
	"net/http"
	"os"

	"github.com/mdlayher/vsock"
)

// NewHTTPClientWithSocketPath creates a client.
// socketPath is a path to the UNIX socket, without unix:// prefix.
func NewHTTPClientWithSocketPath(socketPath string) (*http.Client, error) {
	if _, err := os.Stat(socketPath); err != nil {
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

// NewHTTPClientWithVSockPort creates a client.
// port is the port to use for the vsock.
func NewHTTPClientWithVSockPort(port int) *http.Client {
	hc := &http.Client{
		Transport: &http.Transport{
			Dial: func(_, _ string) (net.Conn, error) {
				return vsock.Dial(2, uint32(port), nil)
			},
		},
	}
	return hc
}
