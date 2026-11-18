// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package guesttoguest

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/mdlayher/netlink"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Run keeps the guest-to-guest routes on the named interfaces until ctx is done.
func Run(ctx context.Context, interfaces []string) error {
	events, err := netlink.Dial(unix.NETLINK_ROUTE, &netlink.Config{
		Groups: unix.RTMGRP_LINK | unix.RTMGRP_IPV4_IFADDR | unix.RTMGRP_IPV4_ROUTE,
	})
	if err != nil {
		return err
	}
	// A separate conn for dumps and writes, so that replies and events never mix.
	requests, err := netlink.Dial(unix.NETLINK_ROUTE, nil)
	if err != nil {
		events.Close()
		return err
	}
	defer requests.Close()
	// Closing the conn unblocks Receive.
	stop := context.AfterFunc(ctx, func() { events.Close() })
	defer func() {
		if stop() {
			events.Close()
		}
	}()

	logrus.Infof("keeping guest-to-guest routes on %v", interfaces)
	reconcile(requests, interfaces)
	for {
		if _, err := events.Receive(); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if !errors.Is(err, unix.ENOBUFS) {
				return err
			}
			logrus.Debug("netlink event socket overflowed, reconciling routes")
		}
		reconcile(requests, interfaces)
	}
}

func reconcile(conn *netlink.Conn, interfaces []string) {
	addrs, err := dumpAddresses(conn)
	if err != nil {
		logrus.WithError(err).Warn("failed to list IPv4 addresses")
		return
	}
	routes, err := dumpRoutes(conn)
	if err != nil {
		logrus.WithError(err).Warn("failed to list IPv4 routes")
		return
	}
	for _, name := range interfaces {
		iface, err := net.InterfaceByName(name)
		if err != nil {
			logrus.WithError(err).Debugf("skipping interface %q", name)
			continue
		}
		for _, r := range MissingRoutes(iface.Index, addrs, routes) {
			desc := fmt.Sprintf("%s via %s dev %s src %s metric %d", r.Dst, r.Gateway, name, r.PrefSrc, r.Priority)
			if err := installRoute(conn, r); err != nil {
				logrus.WithError(err).Warnf("failed to add guest-to-guest route %s", desc)
				continue
			}
			logrus.Infof("added guest-to-guest route %s", desc)
		}
	}
}

func dumpAddresses(conn *netlink.Conn) ([]Address, error) {
	hdr := make([]byte, unix.SizeofIfAddrmsg)
	hdr[0] = unix.AF_INET
	msgs, err := conn.Execute(netlink.Message{
		Header: netlink.Header{Type: unix.RTM_GETADDR, Flags: netlink.Request | netlink.Dump},
		Data:   hdr,
	})
	if err != nil {
		return nil, err
	}
	var addrs []Address
	for _, m := range msgs {
		a, ok, err := parseAddress(m.Data)
		if err != nil {
			return nil, err
		}
		if ok {
			addrs = append(addrs, a)
		}
	}
	return addrs, nil
}

// parseAddress parses an RTM_NEWADDR message body; ok is false for anything but IPv4.
func parseAddress(b []byte) (a Address, ok bool, err error) {
	// struct ifaddrmsg: family, prefixlen, flags, scope (1 byte each), index (4 bytes)
	if len(b) < unix.SizeofIfAddrmsg {
		return Address{}, false, fmt.Errorf("short ifaddrmsg: %d bytes", len(b))
	}
	if b[0] != unix.AF_INET {
		return Address{}, false, nil
	}
	bits := int(b[1])
	index := int(binary.NativeEndian.Uint32(b[4:8]))
	ad, err := netlink.NewAttributeDecoder(b[unix.SizeofIfAddrmsg:])
	if err != nil {
		return Address{}, false, err
	}
	var local, address netip.Addr
	for ad.Next() {
		switch ad.Type() {
		case unix.IFA_LOCAL:
			local, _ = netip.AddrFromSlice(ad.Bytes())
		case unix.IFA_ADDRESS:
			address, _ = netip.AddrFromSlice(ad.Bytes())
		}
	}
	if err := ad.Err(); err != nil {
		return Address{}, false, err
	}
	addr := local
	if !addr.IsValid() {
		addr = address
	}
	if !addr.Is4() {
		return Address{}, false, nil
	}
	return Address{Index: index, Prefix: netip.PrefixFrom(addr, bits)}, true, nil
}

func dumpRoutes(conn *netlink.Conn) ([]Route, error) {
	hdr := make([]byte, unix.SizeofRtMsg)
	hdr[0] = unix.AF_INET
	msgs, err := conn.Execute(netlink.Message{
		Header: netlink.Header{Type: unix.RTM_GETROUTE, Flags: netlink.Request | netlink.Dump},
		Data:   hdr,
	})
	if err != nil {
		return nil, err
	}
	var routes []Route
	for _, m := range msgs {
		r, ok, err := parseRoute(m.Data)
		if err != nil {
			return nil, err
		}
		if ok {
			routes = append(routes, r)
		}
	}
	return routes, nil
}

// parseRoute parses an RTM_NEWROUTE message body; ok is false for anything
// but an IPv4 route in the main table.
func parseRoute(b []byte) (r Route, ok bool, err error) {
	// struct rtmsg: family, dst_len, src_len, tos, table, protocol, scope, type (1 byte each), flags (4 bytes)
	if len(b) < unix.SizeofRtMsg {
		return Route{}, false, fmt.Errorf("short rtmsg: %d bytes", len(b))
	}
	if b[0] != unix.AF_INET {
		return Route{}, false, nil
	}
	dstLen := int(b[1])
	table := uint32(b[4])
	ad, err := netlink.NewAttributeDecoder(b[unix.SizeofRtMsg:])
	if err != nil {
		return Route{}, false, err
	}
	dst := netip.IPv4Unspecified()
	for ad.Next() {
		switch ad.Type() {
		case unix.RTA_TABLE:
			table = ad.Uint32()
		case unix.RTA_DST:
			dst, _ = netip.AddrFromSlice(ad.Bytes())
		case unix.RTA_GATEWAY:
			r.Gateway, _ = netip.AddrFromSlice(ad.Bytes())
		case unix.RTA_OIF:
			r.OIF = int(ad.Uint32())
		case unix.RTA_PRIORITY:
			r.Priority = ad.Uint32()
		case unix.RTA_PREFSRC:
			r.PrefSrc, _ = netip.AddrFromSlice(ad.Bytes())
		}
	}
	if err := ad.Err(); err != nil {
		return Route{}, false, err
	}
	if table != unix.RT_TABLE_MAIN {
		return Route{}, false, nil
	}
	r.Dst = netip.PrefixFrom(dst, dstLen)
	return r, true, nil
}

func installRoute(conn *netlink.Conn, r Route) error {
	data, err := encodeRoute(r)
	if err != nil {
		return err
	}
	_, err = conn.Execute(netlink.Message{
		Header: netlink.Header{
			Type:  unix.RTM_NEWROUTE,
			Flags: netlink.Request | netlink.Acknowledge | netlink.Create | netlink.Replace,
		},
		Data: data,
	})
	return err
}

func encodeRoute(r Route) ([]byte, error) {
	hdr := make([]byte, unix.SizeofRtMsg)
	hdr[0] = unix.AF_INET
	hdr[1] = byte(r.Dst.Bits())
	hdr[4] = unix.RT_TABLE_MAIN
	hdr[5] = unix.RTPROT_STATIC
	hdr[6] = unix.RT_SCOPE_UNIVERSE
	hdr[7] = unix.RTN_UNICAST
	// Mark the gateway on-link, so the kernel accepts the route without a
	// separate route to the gateway.
	binary.NativeEndian.PutUint32(hdr[8:12], unix.RTNH_F_ONLINK)

	ae := netlink.NewAttributeEncoder()
	ae.Uint32(unix.RTA_TABLE, unix.RT_TABLE_MAIN)
	ae.Bytes(unix.RTA_DST, r.Dst.Addr().AsSlice())
	ae.Bytes(unix.RTA_GATEWAY, r.Gateway.AsSlice())
	ae.Uint32(unix.RTA_OIF, uint32(r.OIF))
	ae.Uint32(unix.RTA_PRIORITY, r.Priority)
	ae.Bytes(unix.RTA_PREFSRC, r.PrefSrc.AsSlice())
	attrs, err := ae.Encode()
	if err != nil {
		return nil, err
	}
	return append(hdr, attrs...), nil
}
