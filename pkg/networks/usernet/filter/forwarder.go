// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/containers/gvisor-tap-vsock/pkg/services/forwarder"
	"github.com/containers/gvisor-tap-vsock/pkg/tcpproxy"
	"github.com/sirupsen/logrus"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const linkLocalSubnet = "169.254.0.0/16"

// FilterContext holds pre-parsed values for efficient filtering
// These values are computed once at forwarder creation time to avoid
// repeated string parsing in the hot path (called for every packet).
type FilterContext struct {
	gatewayOne  net.IP // Pre-calculated .1 IP from subnet (allows all TCP/UDP)
	limaGateway net.IP // Pre-parsed Lima gateway IP (allows only UDP:53)
	policy      *Policy
	tracker     *Tracker
}

// newFilterContext creates a filter context with pre-parsed values.
func newFilterContext(pol *Policy, tracker *Tracker, localSubnet, gatewayIP string) *FilterContext {
	ctx := &FilterContext{
		policy:  pol,
		tracker: tracker,
	}

	// Pre-parse .1 from subnet
	if localSubnet != "" {
		_, parsedSubnet, err := net.ParseCIDR(localSubnet)
		if err == nil {
			networkIP := parsedSubnet.IP.To4()
			if networkIP != nil {
				ctx.gatewayOne = net.IPv4(networkIP[0], networkIP[1], networkIP[2], 1)
			}
		}
	}

	// Pre-parse Lima gateway
	if gatewayIP != "" {
		ctx.limaGateway = net.ParseIP(gatewayIP)
	}

	return ctx
}

// FilteredTCPForwarder creates a TCP forwarder with policy filtering
// It checks destination IPs BEFORE NAT, allowing us to block direct IP access.
func FilteredTCPForwarder(s *stack.Stack, nat map[tcpip.Address]tcpip.Address, natLock *sync.Mutex, ec2MetadataAccess bool, pol *Policy, tracker *Tracker, localSubnet, gatewayIP string) *tcp.Forwarder {
	// Pre-parse all gateway IPs once for performance
	filterContext := newFilterContext(pol, tracker, localSubnet, gatewayIP)
	return tcp.NewForwarder(s, 0, 10, func(r *tcp.ForwarderRequest) {
		localAddress := r.ID().LocalAddress
		localPort := r.ID().LocalPort

		// EC2 metadata check
		if (!ec2MetadataAccess) && linkLocal().Contains(localAddress) {
			r.Complete(true)
			return
		}

		// Convert to net.IP for policy checking
		destIP := net.IP(localAddress.AsSlice())

		// Check policy BEFORE NAT - this is where we can see the real destination!
		if !isDestinationAllowed(destIP, localPort, "tcp", filterContext) {
			logrus.Infof("[egress-filter] Blocked TCP connection to %s:%d (policy violation)", destIP, localPort)
			r.Complete(true)
			return
		}

		// Apply NAT translation
		natLock.Lock()
		if replaced, ok := nat[localAddress]; ok {
			localAddress = replaced
		}
		natLock.Unlock()

		// Forward the connection
		var d net.Dialer
		outbound, err := d.DialContext(context.Background(), "tcp", net.JoinHostPort(localAddress.String(), fmt.Sprintf("%d", localPort)))
		if err != nil {
			logrus.Tracef("net.DialContext() = %v", err)
			r.Complete(true)
			return
		}

		var wq waiter.Queue
		ep, tcpErr := r.CreateEndpoint(&wq)
		r.Complete(false)
		if tcpErr != nil {
			if _, ok := tcpErr.(*tcpip.ErrConnectionRefused); ok {
				logrus.Debugf("r.CreateEndpoint() = %v", tcpErr)
			} else {
				logrus.Errorf("r.CreateEndpoint() = %v", tcpErr)
			}
			return
		}

		remote := tcpproxy.DialProxy{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return outbound, nil
			},
		}
		remote.HandleConn(gonet.NewTCPConn(&wq, ep))
	})
}

// FilteredUDPForwarder creates a UDP forwarder with policy filtering.
func FilteredUDPForwarder(s *stack.Stack, nat map[tcpip.Address]tcpip.Address, natLock *sync.Mutex, pol *Policy, tracker *Tracker, localSubnet, gatewayIP string) *udp.Forwarder {
	// Pre-parse all gateway IPs once for performance
	filterContext := newFilterContext(pol, tracker, localSubnet, gatewayIP)
	return udp.NewForwarder(s, func(r *udp.ForwarderRequest) {
		localAddress := r.ID().LocalAddress
		localPort := r.ID().LocalPort

		// Skip DNS - it's handled by gvisor's internal DNS server
		// DNS queries should not be forwarded via net.Dial
		if localPort == 53 {
			// Don't handle this - let it fall through to gvisor's DNS handling
			// which is NOT handled by forwarders
			return
		}

		// Link-local and broadcast check
		if linkLocal().Contains(localAddress) || localAddress == header.IPv4Broadcast {
			return
		}

		// Convert to net.IP for policy checking
		destIP := net.IP(localAddress.AsSlice())

		// Check policy BEFORE NAT
		if !isDestinationAllowed(destIP, localPort, "udp", filterContext) {
			logrus.Infof("[egress-filter] Blocked UDP connection to %s:%d (policy violation)", destIP, localPort)
			return
		}

		// Apply NAT translation
		natLock.Lock()
		if replaced, ok := nat[localAddress]; ok {
			localAddress = replaced
		}
		natLock.Unlock()

		var wq waiter.Queue
		ep, tcpErr := r.CreateEndpoint(&wq)
		if tcpErr != nil {
			if _, ok := tcpErr.(*tcpip.ErrConnectionRefused); ok {
				logrus.Debugf("r.CreateEndpoint() = %v", tcpErr)
			} else {
				logrus.Errorf("r.CreateEndpoint() = %v", tcpErr)
			}
			return
		}

		// Use the UDP proxy from gvisor-tap-vsock
		p, _ := forwarder.NewUDPProxy(&udpConnAdapter{underlying: gonet.NewUDPConn(&wq, ep)}, func() (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(context.Background(), "udp", net.JoinHostPort(localAddress.String(), fmt.Sprintf("%d", localPort)))
		})
		go func() {
			p.Run()
			ep.Close()
		}()
	})
}

// isDestinationAllowed checks if a destination IP:port:protocol is allowed by policy
// This function is called for EVERY TCP/UDP packet, so it's highly optimized:
// 1. Fast-path checks using byte comparisons (no parsing)
// 2. Pre-parsed gateway IPs from FilterContext (no string operations)
// 3. Early returns to minimize work.
func isDestinationAllowed(destIP net.IP, destPort uint16, protocol string, filterContext *FilterContext) bool {
	// Fast-path: IPv4 gateway checks using byte-level comparisons
	// This avoids the overhead of net.IP.Equal() for the most common cases
	// We normalize to 4-byte representation for consistent comparison
	ip4 := destIP.To4()
	if ip4 != nil {
		// Check if destIP matches pre-calculated .1 gateway
		if filterContext.gatewayOne != nil {
			gw1 := filterContext.gatewayOne.To4()
			if gw1 != nil &&
				ip4[0] == gw1[0] && ip4[1] == gw1[1] &&
				ip4[2] == gw1[2] && ip4[3] == gw1[3] {
				// Allow all TCP and UDP to .1
				if protocol == "tcp" || protocol == "udp" {
					return true
				}
			}
		}

		// Check if destIP matches Lima gateway (.2) for DNS
		if filterContext.limaGateway != nil {
			gw2 := filterContext.limaGateway.To4()
			if gw2 != nil &&
				ip4[0] == gw2[0] && ip4[1] == gw2[1] &&
				ip4[2] == gw2[2] && ip4[3] == gw2[3] {
				// Only allow UDP port 53 to Lima gateway
				if protocol == "udp" && destPort == 53 {
					return true
				}
			}
		}
	}

	// Also allow loopback and link-local for basic connectivity
	if destIP.IsLoopback() || destIP.IsLinkLocalUnicast() {
		return true
	}

	// Check each policy rule in order
	for _, rule := range filterContext.policy.Rules {
		if ruleMatches(rule, destIP, destPort, protocol, filterContext.tracker) {
			if rule.IsAllowRule() {
				return true
			}
			// Deny rule matched
			return false
		}
	}

	// No rule matched - default deny
	return false
}

// ruleMatches checks if a policy rule matches the given destination.
func ruleMatches(rule PolicyRule, destIP net.IP, destPort uint16, protocol string, tracker *Tracker) bool {
	if rule.MatchesAll() {
		return true
	}

	if rule.Egress == nil {
		return false
	}

	// Check protocol
	if len(rule.Egress.Protocols) > 0 {
		protocolMatches := false
		for _, p := range rule.Egress.Protocols {
			if p == protocol {
				protocolMatches = true
				break
			}
		}
		if !protocolMatches {
			return false
		}
	}

	// Check port
	if len(rule.Egress.Ports) > 0 {
		portMatches := false
		for _, portStr := range rule.Egress.Ports {
			start, end, err := parsePortRange(portStr)
			if err != nil {
				continue
			}
			if destPort >= start && destPort <= end {
				portMatches = true
				break
			}
		}
		if !portMatches {
			return false
		}
	}

	// Check IP ranges
	if len(rule.Egress.IPs) > 0 {
		ipMatches := false
		for _, ipStr := range rule.Egress.IPs {
			_, ipNet, err := net.ParseCIDR(ipStr)
			if err != nil {
				// Try as single IP
				if parsedIP := net.ParseIP(ipStr); parsedIP != nil {
					if parsedIP.Equal(destIP) {
						ipMatches = true
						break
					}
				}
				continue
			}
			if ipNet.Contains(destIP) {
				ipMatches = true
				break
			}
		}
		if ipMatches {
			return true // IP matched, rule applies
		}
	}

	// Check domain patterns (via DNS tracker)
	if len(rule.Egress.Domains) > 0 {
		domains := tracker.GetDomainsForIP(destIP)
		for _, domain := range domains {
			for _, pattern := range rule.Egress.Domains {
				if matchesPattern(domain, pattern) {
					return true // Domain matched, rule applies
				}
			}
		}
	}

	// If rule has IPs or domains but none matched, rule doesn't apply
	if len(rule.Egress.IPs) > 0 || len(rule.Egress.Domains) > 0 {
		return false
	}

	// Rule has protocol/port restrictions but no IP/domain restrictions
	// If we got here, protocol and port matched
	return true
}

func linkLocal() *tcpip.Subnet {
	_, parsedSubnet, _ := net.ParseCIDR(linkLocalSubnet)
	subnet, _ := tcpip.NewSubnet(tcpip.AddrFromSlice(parsedSubnet.IP), tcpip.MaskFromBytes(parsedSubnet.Mask))
	return &subnet
}

// udpConnAdapter wraps gonet.UDPConn to satisfy the udpConn interface needed by forwarder.NewUDPProxy
// Unfortunately the forwarder package doesn't export its udpConn and autoStoppingListener types.
type udpConnAdapter struct {
	underlying *gonet.UDPConn
}

func (u *udpConnAdapter) ReadFrom(b []byte) (int, net.Addr, error) {
	return u.underlying.ReadFrom(b)
}

func (u *udpConnAdapter) WriteTo(b []byte, addr net.Addr) (int, error) {
	return u.underlying.WriteTo(b, addr)
}

func (u *udpConnAdapter) SetReadDeadline(t time.Time) error {
	return u.underlying.SetReadDeadline(t)
}

func (u *udpConnAdapter) Close() error {
	return u.underlying.Close()
}
