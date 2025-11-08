// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/gvisor-tap-vsock/pkg/virtualnetwork"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

// FilteredVirtualNetwork wraps a virtualnetwork.VirtualNetwork with policy filtering.
type FilteredVirtualNetwork struct {
	vn         *virtualnetwork.VirtualNetwork
	dnsTracker *Tracker
	policy     *Policy
	stack      *stack.Stack
}

// Filter wraps an existing virtual network with policy filtering using a pre-parsed policy.
// This allows the virtual network to be created first, then optionally wrapped with filtering.
// The config parameter should be the same Configuration used to create the VirtualNetwork.
func Filter(vn *virtualnetwork.VirtualNetwork, config *types.Configuration, pol *Policy) (*FilteredVirtualNetwork, error) {
	if pol == nil {
		return nil, errors.New("policy cannot be nil")
	}

	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	// Get the network stack from the virtual network using reflection
	st, err := getStackFromVirtualNetwork(vn)
	if err != nil {
		return nil, fmt.Errorf("failed to get stack from virtual network: %w", err)
	}

	// Create DNS tracker for domain resolution
	dnsTracker := NewTracker()

	// Seed tracker with Lima internal domains (host.lima.internal, subnet.lima.internal)
	if err := dnsTracker.SeedLimaInternalDomains(config.Subnet, config.GatewayIP); err != nil {
		return nil, fmt.Errorf("failed to seed Lima internal domains: %w", err)
	}

	// Apply policy filtering to the stack (iptables for DNS filtering)
	if err := ApplyPolicy(st, pol, dnsTracker, config.Subnet, config.GatewayIP); err != nil {
		return nil, fmt.Errorf("failed to apply policy: %w", err)
	}

	// Install filtered forwarders for TCP/UDP traffic
	// This provides pre-NAT filtering at the transport layer
	if err := installFilteredForwarders(st, config, pol, dnsTracker); err != nil {
		return nil, fmt.Errorf("failed to install filtered forwarders: %w", err)
	}

	// NOTE: ICMP filtering by destination IP is not currently supported due to gvisor architecture.
	// Unlike TCP/UDP which use forwarders that see pre-NAT destinations, ICMP packets are NAT'd
	// before reaching any hook point where we can filter them. The policy can only allow/deny
	// ALL ICMP traffic via the "allow-icmp" rule, not filter by specific destinations.

	fvn := &FilteredVirtualNetwork{
		vn:         vn,
		dnsTracker: dnsTracker,
		policy:     pol,
		stack:      st,
	}

	// Start a background goroutine to clean up expired DNS entries
	go fvn.cleanupExpiredDNS()

	return fvn, nil
}

// installFilteredForwarders replaces the default TCP/UDP forwarders with filtered versions
// This allows us to see and filter on actual destination IPs before NAT.
func installFilteredForwarders(st *stack.Stack, config *types.Configuration, pol *Policy, tracker *Tracker) error {
	// Parse NAT table
	var natLock sync.Mutex
	nat := parseNATTable(config)

	// Install filtered TCP forwarder
	tcpForwarder := FilteredTCPForwarder(st, nat, &natLock, config.Ec2MetadataAccess, pol, tracker, config.Subnet, config.GatewayIP)
	st.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

	// Install filtered UDP forwarder
	udpForwarder := FilteredUDPForwarder(st, nat, &natLock, pol, tracker, config.Subnet, config.GatewayIP)
	st.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)

	return nil
}

func parseNATTable(configuration *types.Configuration) map[tcpip.Address]tcpip.Address {
	translation := make(map[tcpip.Address]tcpip.Address)
	for source, destination := range configuration.NAT {
		translation[tcpip.AddrFrom4Slice(net.ParseIP(source).To4())] = tcpip.AddrFrom4Slice(net.ParseIP(destination).To4())
	}
	return translation
}

// ApplyPolicy applies the policy to a network stack.
func ApplyPolicy(st *stack.Stack, pol *Policy, dnsTracker *Tracker, localSubnet, gatewayIP string) error {
	ipt := st.IPTables()

	// Build and apply IPv4 filter table
	ipv4Table, err := BuildFilterTable(pol, dnsTracker, localSubnet, gatewayIP, false)
	if err != nil {
		return fmt.Errorf("failed to build IPv4 filter table: %w", err)
	}
	ipt.ForceReplaceTable(stack.FilterID, ipv4Table, false)

	// Build and apply IPv6 filter table
	ipv6Table, err := BuildFilterTable(pol, dnsTracker, localSubnet, gatewayIP, true)
	if err != nil {
		return fmt.Errorf("failed to build IPv6 filter table: %w", err)
	}
	ipt.ForceReplaceTable(stack.FilterID, ipv6Table, true)

	return nil
}

// cleanupExpiredDNS periodically cleans up expired DNS records.
func (fvn *FilteredVirtualNetwork) cleanupExpiredDNS() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		fvn.dnsTracker.CleanExpired()
	}
}

// VirtualNetwork returns the underlying virtual network
// This allows the filtered network to be used anywhere a *virtualnetwork.VirtualNetwork is expected.
func (fvn *FilteredVirtualNetwork) VirtualNetwork() *virtualnetwork.VirtualNetwork {
	return fvn.vn
}

// getStackFromVirtualNetwork uses reflection to access the unexported stack field.
func getStackFromVirtualNetwork(vn *virtualnetwork.VirtualNetwork) (*stack.Stack, error) {
	vnValue := reflect.ValueOf(vn).Elem()
	stackField := vnValue.FieldByName("stack")

	if !stackField.IsValid() {
		return nil, errors.New("stack field not found in VirtualNetwork")
	}

	// Make the field accessible using unsafe
	stackField = reflect.NewAt(stackField.Type(), unsafe.Pointer(stackField.UnsafeAddr())).Elem()

	st, ok := stackField.Interface().(*stack.Stack)
	if !ok {
		return nil, errors.New("stack field is not of type *stack.Stack")
	}

	if st == nil {
		return nil, errors.New("stack is nil")
	}

	return st, nil
}
