// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package httpclientutil

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"
)

func TestIsNonPublicAddr(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":       true,
		"::1":             true,
		"0.0.0.0":         true,
		"169.254.169.254": true, // cloud metadata / link-local
		"10.0.0.1":        true,
		"172.16.0.1":      true,
		"192.168.1.1":     true,
		"fc00::1":         true, // unique-local
		"fe80::1":         true, // link-local
		"8.8.8.8":         false,
		"1.1.1.1":         false,
		"93.184.216.34":   false,
	}
	for s, want := range cases {
		ip := net.ParseIP(s)
		assert.Assert(t, ip != nil, "failed to parse %q", s)
		assert.Equal(t, isNonPublicAddr(ip), want, "isNonPublicAddr(%q)", s)
	}
}

// TestRedirectSafeClientRefusesLoopbackRedirect verifies that a server-driven
// redirect into a non-public address is refused, while a direct request to the
// same (loopback) server is still allowed.
func TestRedirectSafeClientRefusesLoopbackRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx := context.Background()

	// A server-driven redirect into the loopback server must be refused.
	resp, err := Get(ctx, RedirectSafeClient(), srv.URL+"/redirect")
	if resp != nil {
		resp.Body.Close()
	}
	assert.ErrorContains(t, err, "refusing redirect to non-public address")

	// A direct operator-supplied URL to the same loopback server is not
	// restricted, only server-driven redirects into non-public space are.
	resp, err = Get(ctx, RedirectSafeClient(), srv.URL+"/final")
	assert.NilError(t, err)
	resp.Body.Close()
}
