// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package guesttoguest

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

func TestParseRouteRoundTrip(t *testing.T) {
	want := Route{
		Dst:      netip.MustParsePrefix("192.168.64.0/24"),
		Gateway:  netip.MustParseAddr("192.168.64.1"),
		OIF:      3,
		Priority: 0,
		PrefSrc:  netip.MustParseAddr("192.168.64.6"),
	}
	b, err := encodeRoute(want)
	assert.NilError(t, err)
	got, ok, err := parseRoute(b)
	assert.NilError(t, err)
	assert.Assert(t, ok)
	assert.Equal(t, got, want)
}

func TestParseRouteDefault(t *testing.T) {
	hdr := make([]byte, unix.SizeofRtMsg)
	hdr[0] = unix.AF_INET
	hdr[4] = unix.RT_TABLE_MAIN
	ae := netlink.NewAttributeEncoder()
	ae.Bytes(unix.RTA_GATEWAY, []byte{192, 168, 64, 1})
	ae.Uint32(unix.RTA_OIF, 3)
	ae.Uint32(unix.RTA_PRIORITY, 100)
	attrs, err := ae.Encode()
	assert.NilError(t, err)
	got, ok, err := parseRoute(append(hdr, attrs...))
	assert.NilError(t, err)
	assert.Assert(t, ok)
	assert.Equal(t, got, Route{Dst: defaultDst, Gateway: netip.MustParseAddr("192.168.64.1"), OIF: 3, Priority: 100})
}

func TestParseRouteOtherTable(t *testing.T) {
	hdr := make([]byte, unix.SizeofRtMsg)
	hdr[0] = unix.AF_INET
	hdr[4] = unix.RT_TABLE_COMPAT
	ae := netlink.NewAttributeEncoder()
	ae.Uint32(unix.RTA_TABLE, unix.RT_TABLE_LOCAL)
	attrs, err := ae.Encode()
	assert.NilError(t, err)
	_, ok, err := parseRoute(append(hdr, attrs...))
	assert.NilError(t, err)
	assert.Assert(t, !ok)
}

func TestParseAddress(t *testing.T) {
	hdr := make([]byte, unix.SizeofIfAddrmsg)
	hdr[0] = unix.AF_INET
	hdr[1] = 24
	binary.NativeEndian.PutUint32(hdr[4:8], 3)
	ae := netlink.NewAttributeEncoder()
	ae.Bytes(unix.IFA_ADDRESS, []byte{192, 168, 64, 6})
	ae.Bytes(unix.IFA_LOCAL, []byte{192, 168, 64, 6})
	attrs, err := ae.Encode()
	assert.NilError(t, err)
	got, ok, err := parseAddress(append(hdr, attrs...))
	assert.NilError(t, err)
	assert.Assert(t, ok)
	assert.Equal(t, got, Address{Index: 3, Prefix: netip.MustParsePrefix("192.168.64.6/24")})
}
