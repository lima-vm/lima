package httpclientutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	winio "github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/lima-vm/lima/pkg/windows"
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

// NewHTTPClientWithVSockPort creates a client for a vsock port.
func NewHTTPClientWithVSockPort(instanceName string, port int) (*http.Client, error) {
	VMIDStr, err := windows.GetInstanceVMID(fmt.Sprintf("lima-%s", instanceName))
	if err != nil {
		return nil, err
	}
	VMIDGUID, err := guid.FromString(VMIDStr)
	if err != nil {
		return nil, err
	}

	serviceGUID, err := guid.FromString(fmt.Sprintf("%x%s", port, windows.MagicVSOCKSuffix))
	if err != nil {
		return nil, err
	}

	sockAddr := &winio.HvsockAddr{
		VMID:      VMIDGUID,
		ServiceID: serviceGUID,
	}

	hc := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return winio.Dial(ctx, sockAddr)
			},
		},
	}
	return hc, nil
}
