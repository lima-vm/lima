//go:build windows
// +build windows

package client

import (
	"net/http"

	"github.com/lima-vm/lima/pkg/httpclientutil"
)

func newVSockGuestAgentClient(port int, instanceName string) (*http.Client, error) {
	hc, err := httpclientutil.NewHTTPClientWithVSockPort(instanceName, port)
	if err != nil {
		return nil, err
	}

	return hc, nil
}
