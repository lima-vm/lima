// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package guesttoguest

import (
	"net/netip"
	"slices"
	"testing"

	"gotest.tools/v3/assert"
)

func TestMissingRoutes(t *testing.T) {
	const lima0, eth0 = 3, 2
	subnet := netip.MustParsePrefix("192.168.64.0/24")
	gw := netip.MustParseAddr("192.168.64.1")
	self := netip.MustParseAddr("192.168.64.6")
	addr := Address{Index: lima0, Prefix: netip.MustParsePrefix("192.168.64.6/24")}
	onLink := Route{Dst: subnet, OIF: lima0, Priority: 100, PrefSrc: self}
	defaultVia := func(gw string, oif int, prio uint32) Route {
		return Route{Dst: defaultDst, Gateway: netip.MustParseAddr(gw), OIF: oif, Priority: prio}
	}
	want := Route{Dst: subnet, Gateway: gw, OIF: lima0, Priority: 0, PrefSrc: self}

	tests := []struct {
		name   string
		addrs  []Address
		routes []Route
		want   []Route
	}{
		{
			name:   "default route on the interface",
			addrs:  []Address{addr},
			routes: []Route{onLink, defaultVia("192.168.64.1", lima0, 100)},
			want:   []Route{want},
		},
		{
			name:   "no default route",
			addrs:  []Address{addr},
			routes: []Route{onLink},
		},
		{
			name:   "default route on another interface",
			addrs:  []Address{addr},
			routes: []Route{onLink, defaultVia("192.168.5.2", eth0, 200)},
		},
		{
			name:   "gateway outside the subnet",
			addrs:  []Address{addr},
			routes: []Route{onLink, defaultVia("10.0.0.1", lima0, 100)},
		},
		{
			name:   "route already present",
			addrs:  []Address{addr},
			routes: []Route{onLink, defaultVia("192.168.64.1", lima0, 100), want},
		},
		{
			name:   "present with a different priority",
			addrs:  []Address{addr},
			routes: []Route{onLink, defaultVia("192.168.64.1", lima0, 100), {Dst: subnet, Gateway: gw, OIF: lima0, Priority: 100, PrefSrc: self}},
			want:   []Route{want},
		},
		{
			name:   "on-link route at priority 0",
			addrs:  []Address{addr},
			routes: []Route{{Dst: subnet, OIF: lima0, Priority: 0, PrefSrc: self}, defaultVia("192.168.64.1", lima0, 100)},
			want:   []Route{want},
		},
		{
			name:   "present with a different source",
			addrs:  []Address{addr},
			routes: []Route{onLink, defaultVia("192.168.64.1", lima0, 100), {Dst: subnet, Gateway: gw, OIF: lima0, PrefSrc: netip.MustParseAddr("192.168.64.7")}},
		},
		{
			name:   "present on another interface",
			addrs:  []Address{addr},
			routes: []Route{onLink, defaultVia("192.168.64.1", lima0, 100), {Dst: subnet, Gateway: gw, OIF: eth0, PrefSrc: netip.MustParseAddr("192.168.64.7")}},
		},
		{
			name:   "two addresses in the subnet",
			addrs:  []Address{addr, {Index: lima0, Prefix: netip.MustParsePrefix("192.168.64.7/24")}},
			routes: []Route{onLink, defaultVia("192.168.64.1", lima0, 100)},
			want:   []Route{want},
		},
		{
			name:   "two default routes, lowest metric wins",
			addrs:  []Address{addr},
			routes: []Route{onLink, defaultVia("192.168.64.2", lima0, 200), defaultVia("192.168.64.1", lima0, 100)},
			want:   []Route{want},
		},
		{
			name:   "/32 address",
			addrs:  []Address{{Index: lima0, Prefix: netip.MustParsePrefix("192.168.64.6/32")}},
			routes: []Route{defaultVia("192.168.64.1", lima0, 100)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MissingRoutes(lima0, tt.addrs, tt.routes)
			assert.Assert(t, slices.Equal(got, tt.want), "got %+v, want %+v", got, tt.want)
		})
	}
}
