// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"net"
	"strings"
	"sync"
	"time"
)

const (
	// MaxDNSRecords is the maximum number of DNS records to track.
	// This prevents unbounded memory growth in long-running processes.
	MaxDNSRecords = 10000
)

// DNSRecord represents a DNS query result with TTL.
type DNSRecord struct {
	Domain   string
	IPs      []net.IP
	ExpireAt time.Time
}

// Tracker tracks domain to IP mappings from DNS queries.
type Tracker struct {
	mu         sync.RWMutex
	records    map[string]*DNSRecord // domain -> record
	limaSubnet *net.IPNet            // subnet.lima.internal CIDR (single special case)
}

// NewTracker creates a new DNS tracker.
func NewTracker() *Tracker {
	return &Tracker{
		records: make(map[string]*DNSRecord),
	}
}

// SeedLimaInternalDomains pre-populates the tracker with Lima internal domains.
// - subnet.lima.internal -> the entire Lima subnet (e.g., 192.168.100.0/24)
// - host.lima.internal -> the Lima gateway (e.g., 192.168.100.2)
func (t *Tracker) SeedLimaInternalDomains(subnet, gatewayIP string) error {
	if subnet == "" {
		return nil
	}

	_, subnetNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return err
	}

	t.mu.Lock()
	t.limaSubnet = subnetNet
	t.mu.Unlock()

	// Pre-seed host.lima.internal with the gateway IP
	// This is needed because gvisor's internal DNS resolves *.lima.internal
	// domains internally, so the DNS snooper never sees the responses
	if gatewayIP != "" {
		gateway := net.ParseIP(gatewayIP)
		if gateway != nil {
			t.addPreSeededRecord("host.lima.internal", []net.IP{gateway})
		}
	}

	return nil
}

// addPreSeededRecord adds a pre-seeded record that never expires (ExpireAt = zero time).
func (t *Tracker) addPreSeededRecord(domain string, ips []net.IP) {
	t.mu.Lock()
	defer t.mu.Unlock()

	domain = strings.ToLower(domain)

	t.records[domain] = &DNSRecord{
		Domain:   domain,
		IPs:      ips,
		ExpireAt: time.Time{}, // zero = never expires
	}
}

// IsPreSeeded returns true if the domain was pre-seeded.
func (t *Tracker) IsPreSeeded(domain string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	domain = strings.ToLower(domain)

	// Check if it's subnet.lima.internal
	if domain == "subnet.lima.internal" {
		return t.limaSubnet != nil
	}

	// Check if it's in records with zero ExpireAt
	record, ok := t.records[domain]
	return ok && record.ExpireAt.IsZero()
}

// AddRecord adds or updates a DNS record (from observed DNS responses).
func (t *Tracker) AddRecord(domain string, ips []net.IP, ttl time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	domain = strings.ToLower(domain)

	// If at capacity and this is a new domain, clean up expired entries first
	if _, exists := t.records[domain]; !exists && len(t.records) >= MaxDNSRecords {
		t.cleanExpiredLocked()

		// If still at capacity after cleanup, remove oldest entry
		if len(t.records) >= MaxDNSRecords {
			t.removeOldestLocked()
		}
	}

	t.records[domain] = &DNSRecord{
		Domain:   domain,
		IPs:      ips,
		ExpireAt: time.Now().Add(ttl),
	}
}

// GetIPs returns all IPs for a domain, or nil if not found/expired.
func (t *Tracker) GetIPs(domain string) []net.IP {
	t.mu.RLock()
	defer t.mu.RUnlock()

	domain = strings.ToLower(domain)

	// Special case: subnet.lima.internal returns the subnet network IP
	if domain == "subnet.lima.internal" && t.limaSubnet != nil {
		return []net.IP{t.limaSubnet.IP}
	}

	record, ok := t.records[domain]
	if !ok {
		return nil
	}
	// Skip expired records
	if !record.ExpireAt.IsZero() && time.Now().After(record.ExpireAt) {
		return nil
	}
	return record.IPs
}

// GetIPsForPattern returns all IPs matching a domain pattern (supports wildcards).
// Example: "*.example.com" matches "api.example.com", "cdn.example.com".
func (t *Tracker) GetIPsForPattern(pattern string) []net.IP {
	t.mu.RLock()
	defer t.mu.RUnlock()

	pattern = strings.ToLower(pattern)
	var allIPs []net.IP
	seenIPs := make(map[string]bool)
	now := time.Now()

	for domain, record := range t.records {
		// Skip expired records
		if !record.ExpireAt.IsZero() && now.After(record.ExpireAt) {
			continue
		}

		// Check if domain matches pattern
		if matchesPattern(domain, pattern) {
			for _, ip := range record.IPs {
				ipStr := ip.String()
				if !seenIPs[ipStr] {
					seenIPs[ipStr] = true
					allIPs = append(allIPs, ip)
				}
			}
		}
	}

	return allIPs
}

// GetDomainsForIP returns all domains that resolve to the given IP (reverse lookup).
func (t *Tracker) GetDomainsForIP(ip net.IP) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var domains []string
	now := time.Now()

	// Check if IP is in the Lima subnet (subnet.lima.internal)
	if t.limaSubnet != nil && t.limaSubnet.Contains(ip) {
		domains = append(domains, "subnet.lima.internal")
	}

	for domain, record := range t.records {
		// Skip expired records
		if !record.ExpireAt.IsZero() && now.After(record.ExpireAt) {
			continue
		}

		// Check if this domain resolves to the given IP (exact match)
		for _, recordIP := range record.IPs {
			if recordIP.Equal(ip) {
				domains = append(domains, domain)
				break
			}
		}
	}

	return domains
}

// CleanExpired removes expired DNS records.
func (t *Tracker) CleanExpired() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cleanExpiredLocked()
}

// cleanExpiredLocked removes expired DNS records (must hold lock).
func (t *Tracker) cleanExpiredLocked() {
	now := time.Now()
	for domain, record := range t.records {
		// Skip expired records
		if !record.ExpireAt.IsZero() && now.After(record.ExpireAt) {
			delete(t.records, domain)
		}
	}
}

// removeOldestLocked removes the record with the earliest expiration time (must hold lock).
func (t *Tracker) removeOldestLocked() {
	if len(t.records) == 0 {
		return
	}

	var oldestDomain string
	var oldestExpireAt time.Time
	first := true

	for domain, record := range t.records {
		// Skip pre-seeded records
		if record.ExpireAt.IsZero() {
			continue
		}
		if first || record.ExpireAt.Before(oldestExpireAt) {
			oldestDomain = domain
			oldestExpireAt = record.ExpireAt
			first = false
		}
	}

	if oldestDomain != "" {
		delete(t.records, oldestDomain)
	}
}

// matchesPattern checks if a domain matches a pattern with wildcard support.
// Pattern examples:
//   - "example.com" matches exactly "example.com"
//   - "*.example.com" matches "api.example.com", "cdn.example.com", but NOT "example.com"
//   - "*" matches everything
func matchesPattern(domain, pattern string) bool {
	// Exact match
	if domain == pattern {
		return true
	}

	// Match all
	if pattern == "*" {
		return true
	}

	// Wildcard pattern
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[2:] // Remove "*."
		// Domain must end with the suffix and have at least one more label
		if strings.HasSuffix(domain, "."+suffix) {
			return true
		}
	}

	return false
}
