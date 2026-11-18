// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package guesttoguest keeps a route to the vzNAT subnet via the host gateway,
// because macOS blocks direct traffic between vzNAT guests on the host's bridge.
package guesttoguest

import (
	"net/netip"
	"slices"
)

// Address is an address assigned to a link, e.g. 192.168.64.6/24.
type Address struct {
	Index  int
	Prefix netip.Prefix
}

// Route is an IPv4 route in the main table.
type Route struct {
	Dst      netip.Prefix
	Gateway  netip.Addr // invalid when the route has no gateway
	OIF      int
	Priority uint32
	PrefSrc  netip.Addr
}

var defaultDst = netip.PrefixFrom(netip.IPv4Unspecified(), 0)

// MissingRoutes returns the routes to add to the interface with the given
// index. Each route sends traffic for one IPv4 subnet of the interface
// through the interface's default gateway. MissingRoutes leaves out a subnet
// that already has a priority-0 route through that gateway, whatever the
// route's source or interface.
//
// The routes use priority 0, so they win over the kernel's on-link route to
// the same subnet. They also set the interface address as the preferred
// source, so the kernel deletes them when that address goes away.
func MissingRoutes(index int, addrs []Address, routes []Route) []Route {
	var gateway netip.Addr
	var gatewayPriority uint32
	for _, r := range routes {
		if r.OIF != index || r.Dst != defaultDst || !r.Gateway.IsValid() {
			continue
		}
		if !gateway.IsValid() || r.Priority < gatewayPriority {
			gateway = r.Gateway
			gatewayPriority = r.Priority
		}
	}
	if !gateway.IsValid() {
		return nil
	}
	var missing []Route
	for _, a := range addrs {
		if a.Index != index || !a.Prefix.Addr().Is4() || a.Prefix.Bits() >= 31 {
			continue
		}
		subnet := a.Prefix.Masked()
		if !subnet.Contains(gateway) {
			continue
		}
		// The watcher installs each route with the netlink replace flag, which
		// overwrites an existing route to the same subnet at the same
		// priority. So if two addresses in the subnet, or two interfaces on
		// it, each wanted its own route, each install would overwrite the
		// other's route. The netlink event from that install would then
		// trigger the next one, without end.
		if slices.ContainsFunc(routes, func(r Route) bool {
			return r.Dst == subnet && r.Gateway == gateway && r.Priority == 0
		}) || slices.ContainsFunc(missing, func(r Route) bool { return r.Dst == subnet }) {
			continue
		}
		missing = append(missing, Route{
			Dst:      subnet,
			Gateway:  gateway,
			OIF:      index,
			Priority: 0,
			PrefSrc:  a.Prefix.Addr(),
		})
	}
	return missing
}
