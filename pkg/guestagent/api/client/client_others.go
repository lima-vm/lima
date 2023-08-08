//go:build !windows
// +build !windows

package client

import (
	"net/http"

	"github.com/lima-vm/lima/pkg/httpclientutil"
)

func newVSockGuestAgentClient(port int, _ string) (*http.Client, error) {
	hc := httpclientutil.NewHTTPClientWithVSockPort(port)

	return hc, nil
}
