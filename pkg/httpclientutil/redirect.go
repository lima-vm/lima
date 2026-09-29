// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package httpclientutil

import (
	"errors"
	"fmt"
	"net"
	"net/http"
)

var redirectSafeClient = &http.Client{CheckRedirect: refuseRedirectToNonPublicAddr}

// RedirectSafeClient returns a shared client for fetching content whose URL
// comes from an untrusted source, such as a remote template or an image
// `location`. It behaves like http.DefaultClient, except that it refuses to
// follow a redirect whose target resolves to a loopback, link-local, private,
// or unspecified address. A malicious server can otherwise answer an
// operator-supplied, external-looking URL with a 302 into the host's own
// loopback or the cloud metadata endpoint (169.254.169.254), turning the fetch
// into an SSRF against host-local services. The operator-supplied URL itself is
// not restricted; only server-driven redirects into non-public space are.
func RedirectSafeClient() *http.Client {
	return redirectSafeClient
}

func refuseRedirectToNonPublicAddr(req *http.Request, via []*http.Request) error {
	// The first request has no redirect history and is the operator-supplied
	// URL, so it is left untouched.
	if len(via) == 0 {
		return nil
	}
	// Preserve net/http's default limit of 10 redirects.
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	host := req.URL.Hostname()
	if host == "" {
		return fmt.Errorf("refusing redirect to %q: no host", req.URL.Redacted())
	}
	ips, err := net.DefaultResolver.LookupIP(req.Context(), "ip", host)
	if err != nil {
		return fmt.Errorf("cannot resolve redirect target %q: %w", host, err)
	}
	for _, ip := range ips {
		if isNonPublicAddr(ip) {
			return fmt.Errorf("refusing redirect to non-public address %s (%s)", ip, req.URL.Redacted())
		}
	}
	return nil
}

// isNonPublicAddr reports whether ip is a loopback, unspecified, link-local, or
// private (RFC1918 / RFC4193 unique-local) address.
func isNonPublicAddr(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate()
}
