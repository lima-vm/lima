// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// BuildFilterTable creates a filter table from a policy
// The table will have OUTPUT chain rules for policy filtering
// INPUT and FORWARD chains accept all traffic since we only filter egress
// localSubnet is used to calculate .1 (network gateway) which allows all TCP/UDP traffic
// gatewayIP is the Lima gateway which only allows UDP port 53.
func BuildFilterTable(pol *Policy, dnsTracker *Tracker, localSubnet, gatewayIP string, ipv6 bool) (stack.Table, error) {
	var rules []stack.Rule
	var networkProto tcpip.NetworkProtocolNumber
	if ipv6 {
		networkProto = header.IPv6ProtocolNumber
	} else {
		networkProto = header.IPv4ProtocolNumber
	}

	// Rule 0: Accept all INPUT traffic (we only filter OUTPUT/egress)
	inputAcceptIndex := 0
	rules = append(rules, stack.Rule{
		Filter: emptyFilter(ipv6),
		Target: &stack.AcceptTarget{NetworkProtocol: networkProto},
	})

	// Rule 1: Accept all FORWARD traffic (we only filter OUTPUT/egress)
	forwardAcceptIndex := 1
	rules = append(rules, stack.Rule{
		Filter: emptyFilter(ipv6),
		Target: &stack.AcceptTarget{NetworkProtocol: networkProto},
	})

	// OUTPUT chain starts here
	outputChainStart := len(rules)

	// Add DNS snooper as first OUTPUT rule
	// This tracks DNS responses for FQDN-based connection filtering
	// Note: dnsSnooper never returns true, so packets are never dropped here
	rules = append(rules, stack.Rule{
		Filter:   emptyFilter(ipv6),
		Matchers: []stack.Matcher{&dnsSnooper{tracker: dnsTracker}},
		Target:   &stack.DropTarget{NetworkProtocol: networkProto},
	})

	// Built-in rule: Always allow DHCP for IPv4
	// DHCP is essential for VM boot - without it, the VM cannot get an IP address
	if !ipv6 {
		_, broadcastNet, _ := net.ParseCIDR("255.255.255.255/32")
		rules = append(rules, stack.Rule{
			Filter: emptyFilter(ipv6),
			Matchers: []stack.Matcher{
				&protocolMatcher{protocol: uint8(header.UDPProtocolNumber)},
				&portMatcher{startPort: 67, endPort: 68},
				&ipMatcher{networks: []*net.IPNet{broadcastNet}},
			},
			Target: &stack.AcceptTarget{NetworkProtocol: networkProto},
		})
	}

	// Built-in rule: Allow all TCP and UDP to .1 (network gateway)
	// .1 is the network gateway and needs unrestricted access
	if localSubnet != "" {
		_, parsedSubnet, err := net.ParseCIDR(localSubnet)
		if err == nil {
			// Calculate .1 IP
			networkIP := parsedSubnet.IP.To4()
			if networkIP != nil && !ipv6 {
				gatewayOne := net.IPv4(networkIP[0], networkIP[1], networkIP[2], 1)
				gatewayOneNet := &net.IPNet{IP: gatewayOne, Mask: net.CIDRMask(32, 32)}

				// Allow all TCP to .1
				rules = append(rules, stack.Rule{
					Filter: emptyFilter(ipv6),
					Matchers: []stack.Matcher{
						&protocolMatcher{protocol: uint8(header.TCPProtocolNumber)},
						&ipMatcher{networks: []*net.IPNet{gatewayOneNet}},
					},
					Target: &stack.AcceptTarget{NetworkProtocol: networkProto},
				})

				// Allow all UDP to .1
				rules = append(rules, stack.Rule{
					Filter: emptyFilter(ipv6),
					Matchers: []stack.Matcher{
						&protocolMatcher{protocol: uint8(header.UDPProtocolNumber)},
						&ipMatcher{networks: []*net.IPNet{gatewayOneNet}},
					},
					Target: &stack.AcceptTarget{NetworkProtocol: networkProto},
				})
			}
		}
	}

	// Built-in rule: Allow UDP port 53 to Lima gateway (from config.GatewayIP, typically .2)
	// DNS is handled by gvisor internally, not via forwarder
	if gatewayIP != "" {
		gateway := net.ParseIP(gatewayIP)
		if gateway != nil {
			// Only add the rule if the gateway IP version matches
			isIPv6Gateway := gateway.To4() == nil
			if isIPv6Gateway == ipv6 {
				// Create /32 (or /128 for IPv6) network for gateway
				var gatewayNet *net.IPNet
				if !ipv6 {
					gatewayNet = &net.IPNet{IP: gateway, Mask: net.CIDRMask(32, 32)}
				} else {
					gatewayNet = &net.IPNet{IP: gateway, Mask: net.CIDRMask(128, 128)}
				}

				// Allow UDP port 53 to Lima gateway
				rules = append(rules, stack.Rule{
					Filter: emptyFilter(ipv6),
					Matchers: []stack.Matcher{
						&protocolMatcher{protocol: uint8(header.UDPProtocolNumber)},
						&portMatcher{startPort: 53, endPort: 53},
						&ipMatcher{networks: []*net.IPNet{gatewayNet}},
					},
					Target: &stack.AcceptTarget{NetworkProtocol: networkProto},
				})
			}
		}
	}

	// Build rules from policy (sorted by priority)
	for _, policyRule := range pol.Rules {
		stackRules, err := buildRulesFromPolicy(policyRule, dnsTracker, networkProto)
		if err != nil {
			return stack.Table{}, fmt.Errorf("failed to build rule '%s': %w", policyRule.Name, err)
		}
		rules = append(rules, stackRules...)
	}

	// Add DNS-resolved-only check before final DROP
	// This prevents IP-based escape where users bypass domain filtering by using direct IPs
	// Only blocks IPs that weren't learned from DNS (i.e., direct IP access)
	rules = append(rules, stack.Rule{
		Filter:   emptyFilter(ipv6),
		Matchers: []stack.Matcher{&dnsResolvedOnlyMatcher{tracker: dnsTracker}},
		Target:   &stack.DropTarget{NetworkProtocol: networkProto},
	})

	// ICMP filtering note:
	// Due to gvisor's NAT architecture, we cannot see real ICMP destinations at the OUTPUT chain.
	// All ICMP packets appear to go to the gateway IP (192.168.7.1), making destination-based
	// filtering impossible. ICMP filtering must be done via policy rules (allow/deny all) rather
	// than by destination IP. The policy rules above handle ICMP based on protocol matching.

	// Add default DROP rule at the end (default deny policy for OUTPUT)
	outputDefaultDropIndex := len(rules)
	rules = append(rules, stack.Rule{
		Filter: emptyFilter(ipv6),
		Target: &stack.DropTarget{NetworkProtocol: networkProto},
	})

	// Build the table with proper chain configuration
	table := stack.Table{
		Rules: rules,
		BuiltinChains: [stack.NumHooks]int{
			stack.Prerouting:  stack.HookUnset,
			stack.Input:       inputAcceptIndex,   // INPUT chain: accept all
			stack.Forward:     forwardAcceptIndex, // FORWARD chain: accept all
			stack.Output:      outputChainStart,   // OUTPUT chain: policy filtering
			stack.Postrouting: stack.HookUnset,
		},
		Underflows: [stack.NumHooks]int{
			stack.Prerouting:  stack.HookUnset,
			stack.Input:       inputAcceptIndex,       // Default for INPUT: accept
			stack.Forward:     forwardAcceptIndex,     // Default for FORWARD: accept
			stack.Output:      outputDefaultDropIndex, // Default for OUTPUT: drop
			stack.Postrouting: stack.HookUnset,
		},
	}

	return table, nil
}

// buildRulesFromPolicy converts a single policy rule into one or more stack.Rule
// Multiple rules are created when the policy specifies multiple protocols or ports,
// implementing OR logic (any rule can match) rather than AND logic (all must match).
func buildRulesFromPolicy(policyRule PolicyRule, dnsTracker *Tracker, networkProto tcpip.NetworkProtocolNumber) ([]stack.Rule, error) {
	var rules []stack.Rule

	// Determine the target based on action
	var target stack.Target
	if policyRule.IsAllowRule() {
		target = &stack.AcceptTarget{NetworkProtocol: networkProto}
	} else {
		target = &stack.DropTarget{NetworkProtocol: networkProto}
	}

	// If the rule matches all traffic, create a simple rule
	if policyRule.MatchesAll() {
		rules = append(rules, stack.Rule{
			Filter: emptyFilter(networkProto == header.IPv6ProtocolNumber),
			Target: target,
		})
		return rules, nil
	}

	// Validate domain-based deny rules
	if policyRule.IsDenyRule() && policyRule.Egress != nil && len(policyRule.Egress.Domains) > 0 {
		for _, domain := range policyRule.Egress.Domains {
			if !dnsTracker.IsPreSeeded(domain) {
				return nil, fmt.Errorf("rule '%s': domain '%s' cannot be used in deny rule - only pre-seeded domains work (currently: host.lima.internal, subnet.lima.internal)", policyRule.Name, domain)
			}
		}
	}

	// Collect IP networks from static IPs only
	ipNetworks, err := collectIPNetworks(policyRule.Egress)
	if err != nil {
		return nil, err
	}

	// Check if we have domain-based rules
	hasDomains := len(policyRule.Egress.Domains) > 0

	// Get protocols and ports to create combinations
	protocols := policyRule.Egress.Protocols
	ports := policyRule.Egress.Ports

	// Convert protocol names to numbers
	var protoNums []uint8
	if len(protocols) > 0 {
		for _, proto := range protocols {
			var protoNum uint8
			switch proto {
			case "tcp":
				protoNum = uint8(header.TCPProtocolNumber)
			case "udp":
				protoNum = uint8(header.UDPProtocolNumber)
			case "icmp":
				// Use ICMPv6 for IPv6, ICMPv4 for IPv4
				if networkProto == header.IPv6ProtocolNumber {
					protoNum = uint8(header.ICMPv6ProtocolNumber)
				} else {
					protoNum = uint8(header.ICMPv4ProtocolNumber)
				}
			default:
				return nil, fmt.Errorf("unsupported protocol: %s", proto)
			}
			protoNums = append(protoNums, protoNum)
		}
	}

	// Parse port ranges
	type portRange struct {
		start uint16
		end   uint16
	}
	var portRanges []portRange
	if len(ports) > 0 {
		for _, portStr := range ports {
			start, end, err := parsePortRange(portStr)
			if err != nil {
				return nil, fmt.Errorf("invalid port range '%s': %w", portStr, err)
			}
			portRanges = append(portRanges, portRange{start: start, end: end})
		}
	}

	// Helper function to create matchers list with IP/domain filtering
	createMatchersWithDestination := func(baseMatchers []stack.Matcher) [][]stack.Matcher {
		var matcherSets [][]stack.Matcher

		// If we have static IPs, create a matcher set with IP matcher
		if len(ipNetworks) > 0 {
			m := make([]stack.Matcher, len(baseMatchers)+1)
			m[0] = &ipMatcher{networks: ipNetworks}
			copy(m[1:], baseMatchers)
			matcherSets = append(matcherSets, m)
		}

		// If we have domains, create a matcher set with domain matcher
		if hasDomains {
			m := make([]stack.Matcher, len(baseMatchers)+1)
			m[0] = &domainMatcher{tracker: dnsTracker, patterns: policyRule.Egress.Domains}
			copy(m[1:], baseMatchers)
			matcherSets = append(matcherSets, m)
		}

		// If we have neither IPs nor domains, just return base matchers
		if len(matcherSets) == 0 {
			matcherSets = append(matcherSets, baseMatchers)
		}

		return matcherSets
	}

	// Create rules for all combinations of protocols and ports
	// This implements OR logic: any combination can match
	hasProtos := len(protoNums) > 0
	hasPorts := len(portRanges) > 0

	switch {
	case hasProtos && hasPorts:
		// Create one rule per protocol+port combination
		for _, protoNum := range protoNums {
			for _, pr := range portRanges {
				baseMatchers := []stack.Matcher{
					&protocolMatcher{protocol: protoNum},
					&portMatcher{startPort: pr.start, endPort: pr.end},
				}
				for _, matchers := range createMatchersWithDestination(baseMatchers) {
					rules = append(rules, stack.Rule{
						Filter:   emptyFilter(networkProto == header.IPv6ProtocolNumber),
						Matchers: matchers,
						Target:   target,
					})
				}
			}
		}
	case hasProtos:
		// Create one rule per protocol (no port matching)
		for _, protoNum := range protoNums {
			baseMatchers := []stack.Matcher{&protocolMatcher{protocol: protoNum}}
			for _, matchers := range createMatchersWithDestination(baseMatchers) {
				rules = append(rules, stack.Rule{
					Filter:   emptyFilter(networkProto == header.IPv6ProtocolNumber),
					Matchers: matchers,
					Target:   target,
				})
			}
		}
	case hasPorts:
		// Create one rule per port (no protocol matching - matches TCP and UDP)
		for _, pr := range portRanges {
			baseMatchers := []stack.Matcher{&portMatcher{startPort: pr.start, endPort: pr.end}}
			for _, matchers := range createMatchersWithDestination(baseMatchers) {
				rules = append(rules, stack.Rule{
					Filter:   emptyFilter(networkProto == header.IPv6ProtocolNumber),
					Matchers: matchers,
					Target:   target,
				})
			}
		}
	default:
		// No protocol or port matching, only IP/domain filtering
		for _, matchers := range createMatchersWithDestination(nil) {
			rules = append(rules, stack.Rule{
				Filter:   emptyFilter(networkProto == header.IPv6ProtocolNumber),
				Matchers: matchers,
				Target:   target,
			})
		}
	}

	return rules, nil
}

// collectIPNetworks collects IP networks from static IPs only (not domains)
// Domain filtering is handled separately via domainMatcher.
func collectIPNetworks(match *PolicyMatch) ([]*net.IPNet, error) {
	var ipNetworks []*net.IPNet

	// Collect IP networks from IPs
	if len(match.IPs) > 0 {
		for _, ipStr := range match.IPs {
			_, ipNet, err := net.ParseCIDR(ipStr)
			if err != nil {
				// Try parsing as a single IP
				ip := net.ParseIP(ipStr)
				if ip == nil {
					return nil, fmt.Errorf("invalid IP or CIDR: %s", ipStr)
				}
				// Convert single IP to /32 or /128 CIDR
				if ip.To4() != nil {
					ipNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)}
				} else {
					ipNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
				}
			}
			ipNetworks = append(ipNetworks, ipNet)
		}
	}

	return ipNetworks, nil
}

// emptyFilter returns an empty IP header filter for the given IP version.
func emptyFilter(ipv6 bool) stack.IPHeaderFilter {
	if ipv6 {
		return stack.EmptyFilter6()
	}
	return stack.EmptyFilter4()
}

// protocolMatcher matches TCP, UDP, or ICMP protocols.
type protocolMatcher struct {
	protocol uint8
}

func (*protocolMatcher) Name() string {
	return "protocolMatcher"
}

func (m *protocolMatcher) Match(_ stack.Hook, pkt *stack.PacketBuffer, _, _ string) (matches, hotdrop bool) {
	// Get the network protocol from the packet
	netProto := pkt.NetworkProtocolNumber
	if netProto != header.IPv4ProtocolNumber && netProto != header.IPv6ProtocolNumber {
		return false, false
	}

	// Get transport protocol from the network header
	var transportProto uint8
	if netProto == header.IPv4ProtocolNumber {
		ipv4 := header.IPv4(pkt.NetworkHeader().Slice())
		if len(ipv4) < header.IPv4MinimumSize {
			return false, true // malformed, hotdrop
		}
		transportProto = ipv4.Protocol()
	} else { // IPv6
		ipv6 := header.IPv6(pkt.NetworkHeader().Slice())
		if len(ipv6) < header.IPv6MinimumSize {
			return false, true // malformed, hotdrop
		}
		transportProto = uint8(ipv6.TransportProtocol())
	}

	return transportProto == m.protocol, false
}

// portMatcher matches destination ports (single port or range).
type portMatcher struct {
	startPort uint16
	endPort   uint16
}

func (*portMatcher) Name() string {
	return "portMatcher"
}

func (m *portMatcher) Match(_ stack.Hook, pkt *stack.PacketBuffer, _, _ string) (matches, hotdrop bool) {
	transportHeader := pkt.TransportHeader().Slice()
	if len(transportHeader) < 4 {
		return false, false
	}

	// Try TCP first
	if len(transportHeader) >= header.TCPMinimumSize {
		tcp := header.TCP(transportHeader)
		dstPort := tcp.DestinationPort()
		return dstPort >= m.startPort && dstPort <= m.endPort, false
	}

	// Try UDP
	if len(transportHeader) >= header.UDPMinimumSize {
		udp := header.UDP(transportHeader)
		dstPort := udp.DestinationPort()
		return dstPort >= m.startPort && dstPort <= m.endPort, false
	}

	return false, false
}

// ipMatcher matches destination IP addresses or CIDR ranges.
type ipMatcher struct {
	networks []*net.IPNet
}

func (*ipMatcher) Name() string {
	return "ipMatcher"
}

func (m *ipMatcher) Match(_ stack.Hook, pkt *stack.PacketBuffer, _, _ string) (matches, hotdrop bool) {
	netProto := pkt.NetworkProtocolNumber
	var dstIP net.IP

	switch netProto {
	case header.IPv4ProtocolNumber:
		ipv4 := header.IPv4(pkt.NetworkHeader().Slice())
		if len(ipv4) < header.IPv4MinimumSize {
			return false, true // malformed, hotdrop
		}
		// Get destination IP from header
		dstIP = net.IP(ipv4.DestinationAddressSlice())
	case header.IPv6ProtocolNumber:
		ipv6 := header.IPv6(pkt.NetworkHeader().Slice())
		if len(ipv6) < header.IPv6MinimumSize {
			return false, true // malformed, hotdrop
		}
		// Get destination IP from header
		dstIP = net.IP(ipv6.DestinationAddressSlice())
	default:
		return false, false
	}

	// Check if the destination IP matches any of our networks
	for _, network := range m.networks {
		if network.Contains(dstIP) {
			return true, false
		}
	}

	return false, false
}

// domainMatcher matches packets based on domain patterns using DNS tracking
// It dynamically checks the DNS tracker to see if the destination IP belongs to an allowed domain.
type domainMatcher struct {
	tracker  *Tracker
	patterns []string // domain patterns like "github.com" or "*.github.com"
}

func (*domainMatcher) Name() string {
	return "domainMatcher"
}

func (m *domainMatcher) Match(_ stack.Hook, pkt *stack.PacketBuffer, _, _ string) (matches, hotdrop bool) {
	netProto := pkt.NetworkProtocolNumber
	var dstIP net.IP

	switch netProto {
	case header.IPv4ProtocolNumber:
		ipv4 := header.IPv4(pkt.NetworkHeader().Slice())
		if len(ipv4) < header.IPv4MinimumSize {
			return false, true // malformed, hotdrop
		}
		dstIP = net.IP(ipv4.DestinationAddressSlice())
	case header.IPv6ProtocolNumber:
		ipv6 := header.IPv6(pkt.NetworkHeader().Slice())
		if len(ipv6) < header.IPv6MinimumSize {
			return false, true // malformed, hotdrop
		}
		dstIP = net.IP(ipv6.DestinationAddressSlice())
	default:
		return false, false
	}

	// Look up which domains resolve to this IP
	domains := m.tracker.GetDomainsForIP(dstIP)

	// Check if any of the domains match our patterns
	for _, domain := range domains {
		for _, pattern := range m.patterns {
			if matchDomainPattern(domain, pattern) {
				return true, false
			}
		}
	}

	return false, false
}

// matchDomainPattern wraps matchesPattern for use in domainMatcher.
func matchDomainPattern(domain, pattern string) bool {
	return matchesPattern(domain, pattern)
}

// dnsResolvedOnlyMatcher blocks traffic to IPs that weren't learned from DNS
// This prevents IP-based escape where users bypass domain filtering by using direct IPs.
type dnsResolvedOnlyMatcher struct {
	tracker *Tracker
}

func (*dnsResolvedOnlyMatcher) Name() string {
	return "dnsResolvedOnly"
}

// isPrivateOrLocalIP checks if an IP is private, localhost, or link-local.
func isPrivateOrLocalIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check RFC1918 private networks
	privateNetworks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateNetworks {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

func (m *dnsResolvedOnlyMatcher) Match(_ stack.Hook, pkt *stack.PacketBuffer, _, _ string) (matches, hotdrop bool) {
	netProto := pkt.NetworkProtocolNumber
	var dstIP net.IP

	switch netProto {
	case header.IPv4ProtocolNumber:
		ipv4 := header.IPv4(pkt.NetworkHeader().Slice())
		if len(ipv4) < header.IPv4MinimumSize {
			return false, true // malformed, hotdrop
		}
		dstIP = net.IP(ipv4.DestinationAddressSlice())
	case header.IPv6ProtocolNumber:
		ipv6 := header.IPv6(pkt.NetworkHeader().Slice())
		if len(ipv6) < header.IPv6MinimumSize {
			return false, true // malformed, hotdrop
		}
		dstIP = net.IP(ipv6.DestinationAddressSlice())
	default:
		return false, false
	}

	// Skip checking for private IPs and localhost (these don't need DNS)
	if isPrivateOrLocalIP(dstIP) {
		return false, false
	}

	// Check if this IP was seen in any recent DNS response
	domains := m.tracker.GetDomainsForIP(dstIP)

	// If no domains found, this IP wasn't learned from DNS - block it
	if len(domains) == 0 {
		logrus.Infof("[egress-filter] Blocked direct IP access to: %s (not resolved via DNS)", dstIP)
		return true, false // Match and drop
	}

	// IP was learned from DNS - allow (don't match)
	return false, false
}

// parsePortRange parses a port string like "443" or "8000-9000".
func parsePortRange(portStr string) (start, end uint16, err error) {
	if strings.Contains(portStr, "-") {
		parts := strings.Split(portStr, "-")
		if len(parts) != 2 {
			return 0, 0, fmt.Errorf("invalid port range format: %s", portStr)
		}
		startInt, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid start port: %w", err)
		}
		if startInt < 1 || startInt > 65535 {
			return 0, 0, fmt.Errorf("start port %d out of range (1-65535)", startInt)
		}
		endInt, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end port: %w", err)
		}
		if endInt < 1 || endInt > 65535 {
			return 0, 0, fmt.Errorf("end port %d out of range (1-65535)", endInt)
		}
		if startInt > endInt {
			return 0, 0, fmt.Errorf("start port %d greater than end port %d", startInt, endInt)
		}
		return uint16(startInt), uint16(endInt), nil
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid port: %w", err)
	}
	if port < 1 || port > 65535 {
		return 0, 0, fmt.Errorf("port %d out of range (1-65535)", port)
	}
	return uint16(port), uint16(port), nil
}
