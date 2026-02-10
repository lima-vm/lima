// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vmnet

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#include <ifaddrs.h>
#include <net/if.h>
*/
import (
	"C" //nolint:gocritic // false positive: dupImport: package is imported 2 times under different aliases on... (gocritic)
)

import (
	"log"
	"net"
	"net/netip"
	"slices"
	"syscall"
	"unsafe" //nolint:gocritic // false positive: dupImport: package is imported 2 times under different aliases on... (gocritic)
)

// LookupInterfaceAndTypeByPrefix looks up a network interface by IP prefix.
// It returns the first interface that has the specified prefix.
// If no such interface is found, it returns (nil, nil).
func LookupInterfaceAndTypeByPrefix(prefix netip.Prefix) (*InterfaceWithTypePrefixesRawFlags, error) {
	ifas, err := NewInterfaces()
	if err != nil {
		return nil, err
	}
	for _, ifa := range ifas {
		if ifa.PrefixesContains(prefix) {
			return &ifa, nil
		}
	}
	return nil, nil
}

// NewInterfaces returns a list of network interfaces with their type, prefixes, and raw flags.
// It uses getifaddrs(3) to retrieve the list of interfaces.
// Similar to net.NewInterfaces, but also includes interface type, prefixes, and raw flags.
func NewInterfaces() (Interfaces, error) {
	var ifaddrs *C.struct_ifaddrs
	//nolint:gocritic // false positive: dupSubExpr: suspicious identical LHS and RHS for `==` operator (gocritic)
	if res, err := C.getifaddrs(&ifaddrs); res != 0 && err != nil {
		return nil, err
	}
	defer C.freeifaddrs(ifaddrs)

	entries := make([]InterfaceWithTypePrefixesRawFlags, 0)
	var entry *InterfaceWithTypePrefixesRawFlags
	for ifa := ifaddrs; ifa != nil; ifa = ifa.ifa_next {
		switch ifa.ifa_addr.sa_family {
		case C.AF_LINK:
			entries = append(entries, InterfaceWithTypePrefixesRawFlags{})
			entry = &entries[len(entries)-1]
			entry.Name = C.GoString(ifa.ifa_name)
			entry.Flags = linkFlags(ifa.ifa_flags)
			entry.Prefixes = make([]netip.Prefix, 0)
			entry.rawFlags = uint(ifa.ifa_flags)
			sa := (*syscall.RawSockaddrDatalink)(unsafe.Pointer(ifa.ifa_addr))
			if ifa.ifa_data != nil {
				ifData := (*syscall.IfData)(ifa.ifa_data)
				entry.Index = int(sa.Index)
				entry.MTU = int(ifData.Mtu)
				entry.Type = ifData.Type
			} else {
				// Fallback to use sa_type
				entry.Type = sa.Type
			}
			if sa.Alen > 0 {
				mac := slices.Clone(unsafe.Slice((*byte)(unsafe.Pointer(&sa.Data[sa.Nlen])), sa.Alen))
				entry.HardwareAddr = net.HardwareAddr(mac)
			}
		case C.AF_INET:
			sa := (*syscall.RawSockaddrInet4)(unsafe.Pointer(ifa.ifa_addr))
			mask := (*syscall.RawSockaddrInet4)(unsafe.Pointer(ifa.ifa_netmask))
			ip := netip.AddrFrom4(sa.Addr)
			ones, _ := net.IPMask(mask.Addr[0:4]).Size()
			prefix := netip.PrefixFrom(ip, ones)
			entry.Prefixes = append(entry.Prefixes, prefix)
		case C.AF_INET6:
			sa := (*syscall.RawSockaddrInet6)(unsafe.Pointer(ifa.ifa_addr))
			mask := (*syscall.RawSockaddrInet6)(unsafe.Pointer(ifa.ifa_netmask))
			ip := netip.AddrFrom16(sa.Addr)
			ones, _ := net.IPMask(mask.Addr[0:16]).Size()
			prefix := netip.PrefixFrom(ip, ones)
			entry.Prefixes = append(entry.Prefixes, prefix)
		default:
			log.Printf("Skipping interface %s with sa_family %d", C.GoString(ifa.ifa_name), ifa.ifa_addr.sa_family)
		}
	}
	return entries, nil
}

// Interfaces is a slice of InterfaceWithTypePrefixesRawFlags.
type Interfaces []InterfaceWithTypePrefixesRawFlags

// LookupInterface looks up an interface that contains the given [netip.Prefix].
// Returns nil if no such interface is found.
func (ifas Interfaces) LookupInterface(prefix netip.Prefix) *InterfaceWithTypePrefixesRawFlags {
	for _, ifa := range ifas {
		if ifa.PrefixesContains(prefix) {
			return &ifa
		}
	}
	return nil
}

// InterfaceWithTypePrefixesRawFlags extends net.Interface with Type, Prefixes, and RawFlags.
type InterfaceWithTypePrefixesRawFlags struct {
	net.Interface
	rawFlags uint  // syscall.IFF_*
	Type     uint8 // syscall.IFT_*
	Prefixes []netip.Prefix
}

// PrefixesContains checks if the interface has a prefix that contains the given prefix.
func (ifa *InterfaceWithTypePrefixesRawFlags) PrefixesContains(prefix netip.Prefix) bool {
	addr := prefix.Addr()
	return slices.ContainsFunc(ifa.Prefixes, func(p netip.Prefix) bool { return p.Contains(addr) })
}

// linkFlags converts C.uint flags to net.Flags based on net.linkFlags in interface_bsd.go.
func linkFlags(rawFlags C.uint) net.Flags {
	var f net.Flags
	if rawFlags&syscall.IFF_UP != 0 {
		f |= net.FlagUp
	}
	if rawFlags&syscall.IFF_RUNNING != 0 {
		f |= net.FlagRunning
	}
	if rawFlags&syscall.IFF_BROADCAST != 0 {
		f |= net.FlagBroadcast
	}
	if rawFlags&syscall.IFF_LOOPBACK != 0 {
		f |= net.FlagLoopback
	}
	if rawFlags&syscall.IFF_POINTOPOINT != 0 {
		f |= net.FlagPointToPoint
	}
	if rawFlags&syscall.IFF_MULTICAST != 0 {
		f |= net.FlagMulticast
	}
	return f
}
