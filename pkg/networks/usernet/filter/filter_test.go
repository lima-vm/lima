// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/gvisor-tap-vsock/pkg/virtualnetwork"
	"gotest.tools/v3/assert"
)

// Test helpers

func newTestConfig() *types.Configuration {
	return &types.Configuration{
		Debug:             false,
		MTU:               1500,
		Subnet:            "192.168.127.0/24",
		GatewayIP:         "192.168.127.1",
		GatewayMacAddress: "5a:94:ef:e4:0c:dd",
	}
}

func newTestVirtualNetwork(t *testing.T, config *types.Configuration) *virtualnetwork.VirtualNetwork {
	t.Helper()
	vn, err := virtualnetwork.New(config)
	assert.NilError(t, err)
	return vn
}

func newFilteredNetwork(t *testing.T, config *types.Configuration, pol *Policy) *FilteredVirtualNetwork {
	t.Helper()
	vn := newTestVirtualNetwork(t, config)
	fvn, err := Filter(vn, config, pol)
	assert.NilError(t, err)
	return fvn
}

func loadPolicyFromString(t *testing.T, policyYAML string) *Policy {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "policy-*.yaml")
	assert.NilError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(policyYAML)
	assert.NilError(t, err)
	tmpFile.Close()

	pol, err := LoadPolicy(tmpFile.Name())
	assert.NilError(t, err)
	return pol
}

func TestNewWithPolicy_Integration(t *testing.T) {
	pol := loadPolicyFromString(t, `version: "1.0"
rules:
  - name: allow-https
    action: allow
    priority: 10
    egress:
      protocols: [tcp]
      ports: ["443"]
  - name: allow-dns
    action: allow
    priority: 20
    egress:
      protocols: [udp]
      ports: ["53"]
  - name: block-metadata
    action: deny
    priority: 5
    egress:
      ips: [169.254.169.254/32]
  - name: deny-all
    action: deny
    priority: 1000`)

	config := newTestConfig()
	fvn := newFilteredNetwork(t, config, pol)

	assert.Assert(t, fvn != nil)
	assert.Assert(t, fvn.VirtualNetwork() != nil)
	assert.Assert(t, fvn.stack != nil)

	table := fvn.stack.IPTables().GetTable(0, false)
	assert.Assert(t, len(table.Rules) > 0, "Expected rules in filter table")
}

func TestNewWithPolicy_InvalidPolicy(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "policy-*.yaml")
	assert.NilError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(`version: "1.0"
rules:
  - name: test
    action: invalid-action
    priority: 1`)
	assert.NilError(t, err)
	tmpFile.Close()

	_, err = LoadPolicy(tmpFile.Name())
	assert.Assert(t, err != nil, "Expected error for invalid policy")
}

func TestPolicyValidation(t *testing.T) {
	tests := []struct {
		name       string
		policy     string
		wantErrMsg string
	}{
		{"invalid version", `version: "2.0"
rules: [{name: test, action: allow, priority: 1}]`, "unsupported policy version"},
		{"invalid port range", `version: "1.0"
rules: [{name: test, action: allow, priority: 1, egress: {ports: ["99999"]}}]`, "port 99999 out of range"},
		{"invalid port range format", `version: "1.0"
rules: [{name: test, action: allow, priority: 1, egress: {ports: ["8000-7000"]}}]`, "start port 8000 greater than end port 7000"},
		{"invalid IP", `version: "1.0"
rules: [{name: test, action: allow, priority: 1, egress: {ips: ["not-an-ip"]}}]`, "not a valid IP address or CIDR notation"},
		{"invalid CIDR", `version: "1.0"
rules: [{name: test, action: allow, priority: 1, egress: {ips: ["192.168.1.1/33"]}}]`, "not a valid IP address or CIDR notation"},
		{"valid ICMP rule", `version: "1.0"
rules: [{name: test, action: allow, priority: 1, egress: {protocols: [icmp]}}]`, ""},
		{"valid domain-based allow", `version: "1.0"
rules: [{name: test, action: allow, priority: 1, egress: {domains: ["github.com"], ports: ["443"]}}]`, ""},
		{"valid policy", `version: "1.0"
rules: [{name: test, action: allow, priority: 1, egress: {protocols: [tcp], ports: ["443", "8000-9000"], ips: ["192.168.1.0/24", "10.0.0.1"]}}]`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp(t.TempDir(), "policy-*.yaml")
			assert.NilError(t, err)
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.WriteString(tt.policy)
			assert.NilError(t, err)
			tmpFile.Close()

			_, err = LoadPolicy(tmpFile.Name())
			if tt.wantErrMsg != "" {
				assert.Assert(t, err != nil, "Expected error but got none")
				assert.Assert(t, strings.Contains(err.Error(), tt.wantErrMsg),
					"Expected error to contain '%s', got: %v", tt.wantErrMsg, err)
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestNewWithPolicy_MissingPolicyFile(t *testing.T) {
	_, err := LoadPolicy("/nonexistent/policy.yaml")
	assert.Assert(t, err != nil, "Expected error for missing policy file")
}

func TestExamplePolicy(t *testing.T) {
	policyPath := filepath.Join(".", "policy.yaml")
	if _, err := os.Stat(policyPath); err != nil {
		t.Skipf("Example policy file not found: %v", err)
	}

	pol, err := LoadPolicy(policyPath)
	assert.NilError(t, err)
	assert.Equal(t, pol.Version, "1.0")
	assert.Assert(t, len(pol.Rules) > 0, "Expected at least one rule")

	config := newTestConfig()
	fvn := newFilteredNetwork(t, config, pol)
	assert.Assert(t, fvn != nil)
	assert.Assert(t, fvn.VirtualNetwork() != nil)

	t.Logf("Successfully loaded example policy with %d rules", len(pol.Rules))
	for i, rule := range pol.Rules {
		t.Logf("  Rule %d: %s (action=%s, priority=%d)", i+1, rule.Name, rule.Action, rule.Priority)
	}
}

func TestICMPFiltering(t *testing.T) {
	pol := loadPolicyFromString(t, `version: "1.0"
rules:
  - name: allow-icmp
    action: allow
    priority: 10
    egress:
      protocols: [icmp]
  - name: deny-all
    action: deny
    priority: 100`)

	assert.Equal(t, len(pol.Rules), 2)
	assert.Equal(t, pol.Rules[0].Name, "allow-icmp")
	assert.Equal(t, len(pol.Rules[0].Egress.Protocols), 1)
	assert.Equal(t, pol.Rules[0].Egress.Protocols[0], "icmp")

	config := newTestConfig()
	fvn := newFilteredNetwork(t, config, pol)

	table := fvn.stack.IPTables().GetTable(0, false)
	assert.Assert(t, len(table.Rules) > 0, "Expected rules in filter table")
}

func TestDNSTrackingIntegration(t *testing.T) {
	pol := loadPolicyFromString(t, `version: "1.0"
rules:
  - name: allow-github-api
    action: allow
    priority: 10
    egress:
      protocols: [tcp]
      domains: [api.github.com]
      ports: ["443"]
  - name: allow-wildcards
    action: allow
    priority: 20
    egress:
      protocols: [tcp]
      domains: ["*.example.com"]
      ports: ["443"]
  - name: deny-all
    action: deny
    priority: 100`)

	tracker := NewTracker()
	tracker.AddRecord("api.github.com", []net.IP{net.ParseIP("140.82.121.6")}, 300*time.Second)
	tracker.AddRecord("cdn.example.com", []net.IP{net.ParseIP("192.0.2.1")}, 300*time.Second)
	tracker.AddRecord("api.example.com", []net.IP{net.ParseIP("192.0.2.2")}, 300*time.Second)

	ips := tracker.GetIPs("api.github.com")
	assert.Equal(t, len(ips), 1)
	assert.Equal(t, ips[0].String(), "140.82.121.6")

	wildcardIPs := tracker.GetIPsForPattern("*.example.com")
	assert.Equal(t, len(wildcardIPs), 2)

	config := newTestConfig()
	ipv4Table, err := BuildFilterTable(pol, tracker, config.Subnet, config.GatewayIP, false)
	assert.NilError(t, err)
	assert.Assert(t, len(ipv4Table.Rules) > 0, "Expected IPv4 rules")

	ipv6Table, err := BuildFilterTable(pol, tracker, config.Subnet, config.GatewayIP, true)
	assert.NilError(t, err)
	assert.Assert(t, len(ipv6Table.Rules) > 0, "Expected IPv6 rules")
}

func TestDNSTrackerExpiration(t *testing.T) {
	tracker := NewTracker()
	tracker.AddRecord("short-lived.com", []net.IP{net.ParseIP("192.0.2.1")}, 1*time.Millisecond)

	ips := tracker.GetIPs("short-lived.com")
	assert.Equal(t, len(ips), 1, "Expected IP immediately")

	time.Sleep(10 * time.Millisecond)

	ips = tracker.GetIPs("short-lived.com")
	assert.Equal(t, len(ips), 0, "Expected expiration")

	tracker.CleanExpired()
	ips = tracker.GetIPs("short-lived.com")
	assert.Equal(t, len(ips), 0, "Expected cleanup")
}

func TestDNSTrackerSizeLimit(t *testing.T) {
	tracker := NewTracker()

	const testLimit = 100
	for i := range testLimit {
		tracker.AddRecord(fmt.Sprintf("expired%d.example.com", i),
			[]net.IP{net.ParseIP("192.0.2.1")}, 1*time.Millisecond)
	}

	time.Sleep(10 * time.Millisecond)
	tracker.AddRecord("new.example.com", []net.IP{net.ParseIP("192.0.2.2")}, 300*time.Second)

	ips := tracker.GetIPs("new.example.com")
	assert.Equal(t, len(ips), 1, "Expected new record")

	tracker2 := NewTracker()
	for i := range 50 {
		tracker2.AddRecord(fmt.Sprintf("test%d.example.com", i),
			[]net.IP{net.ParseIP("192.0.2.1")}, time.Duration(i+1)*time.Second)
	}

	ips = tracker2.GetIPs("test0.example.com")
	assert.Equal(t, len(ips), 1, "Expected test0 to exist")
}

func TestDNSWildcardPatternMatching(t *testing.T) {
	tracker := NewTracker()
	tracker.AddRecord("example.com", []net.IP{net.ParseIP("192.0.2.1")}, 300*time.Second)
	tracker.AddRecord("api.example.com", []net.IP{net.ParseIP("192.0.2.2")}, 300*time.Second)
	tracker.AddRecord("cdn.example.com", []net.IP{net.ParseIP("192.0.2.3")}, 300*time.Second)
	tracker.AddRecord("api.test.example.com", []net.IP{net.ParseIP("192.0.2.4")}, 300*time.Second)
	tracker.AddRecord("other.com", []net.IP{net.ParseIP("192.0.2.5")}, 300*time.Second)

	tests := []struct {
		pattern string
		want    int
		desc    string
	}{
		{"example.com", 1, "exact match"},
		{"*.example.com", 3, "wildcard subdomains"},
		{"*", 5, "star matches all"},
		{"nonexistent.com", 0, "non-existent"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ips := tracker.GetIPsForPattern(tt.pattern)
			assert.Equal(t, len(ips), tt.want, "Pattern: %s", tt.pattern)
		})
	}
}

func TestIPv6ICMPFiltering(t *testing.T) {
	pol := loadPolicyFromString(t, `version: "1.0"
rules:
  - name: allow-icmp
    action: allow
    priority: 10
    egress:
      protocols: [icmp]
  - name: deny-all
    action: deny
    priority: 100`)

	tracker := NewTracker()
	config := newTestConfig()

	ipv4Table, err := BuildFilterTable(pol, tracker, config.Subnet, config.GatewayIP, false)
	assert.NilError(t, err)
	assert.Assert(t, len(ipv4Table.Rules) > 0, "Expected IPv4 rules")

	ipv6Table, err := BuildFilterTable(pol, tracker, config.Subnet, config.GatewayIP, true)
	assert.NilError(t, err)
	assert.Assert(t, len(ipv6Table.Rules) > 0, "Expected IPv6 rules")
}

func TestIPv6WithDomains(t *testing.T) {
	pol := loadPolicyFromString(t, `version: "1.0"
rules:
  - name: allow-ipv6-site
    action: allow
    priority: 10
    egress:
      protocols: [tcp]
      domains: [ipv6.example.com]
      ports: ["443"]
  - name: deny-all
    action: deny
    priority: 100`)

	tracker := NewTracker()
	tracker.AddRecord("ipv6.example.com", []net.IP{
		net.ParseIP("2001:db8::1"),
		net.ParseIP("2001:db8::2"),
	}, 300*time.Second)

	config := newTestConfig()
	ipv6Table, err := BuildFilterTable(pol, tracker, config.Subnet, config.GatewayIP, true)
	assert.NilError(t, err)
	assert.Assert(t, len(ipv6Table.Rules) > 0, "Expected IPv6 rules")

	ips := tracker.GetIPs("ipv6.example.com")
	assert.Equal(t, len(ips), 2, "Expected 2 IPv6 addresses")
}

func TestDomainBasedDenyRules(t *testing.T) {
	tracker := NewTracker()
	subnet := "192.168.100.0/24"
	gatewayIP := "192.168.100.2"

	// Seed Lima internal domains
	err := tracker.SeedLimaInternalDomains(subnet, gatewayIP)
	assert.NilError(t, err)

	// Test: deny rule with non-pre-seeded domain should fail
	polDenyNonSeeded := loadPolicyFromString(t, `version: "1.0"
rules:
  - name: deny-example
    action: deny
    priority: 10
    egress:
      domains: [example.com]`)

	_, err = BuildFilterTable(polDenyNonSeeded, tracker, subnet, gatewayIP, false)
	assert.Assert(t, err != nil, "Expected error for non-pre-seeded domain in deny rule")
	assert.Assert(t, strings.Contains(err.Error(), "pre-seeded"), "Error should mention pre-seeded: %v", err)

	// Test: deny rule with pre-seeded domain should succeed
	polDenySeeded := loadPolicyFromString(t, `version: "1.0"
rules:
  - name: deny-host
    action: deny
    priority: 10
    egress:
      domains: [host.lima.internal]`)

	table, err := BuildFilterTable(polDenySeeded, tracker, subnet, gatewayIP, false)
	assert.NilError(t, err)
	assert.Assert(t, len(table.Rules) > 0, "Expected rules in filter table")

	// Test: deny rule with both seeded and non-seeded domains should fail
	polDenyMixed := loadPolicyFromString(t, `version: "1.0"
rules:
  - name: deny-mixed
    action: deny
    priority: 10
    egress:
      domains: [host.lima.internal, example.com]`)

	_, err = BuildFilterTable(polDenyMixed, tracker, subnet, gatewayIP, false)
	assert.Assert(t, err != nil, "Expected error for mixed seeded/non-seeded domains")
	assert.Assert(t, strings.Contains(err.Error(), "example.com"), "Error should mention the non-seeded domain")
}

func TestLimaInternalDomains(t *testing.T) {
	tracker := NewTracker()
	subnet := "192.168.100.0/24"
	gatewayIP := "192.168.100.2"

	err := tracker.SeedLimaInternalDomains(subnet, gatewayIP)
	assert.NilError(t, err)

	subnetIPs := tracker.GetIPs("subnet.lima.internal")
	assert.Assert(t, len(subnetIPs) > 0, "subnet.lima.internal should resolve")

	hostIPs := tracker.GetIPs("host.lima.internal")
	assert.Equal(t, len(hostIPs), 1, "host.lima.internal should have one IP")
	assert.Equal(t, hostIPs[0].String(), gatewayIP)

	pol := loadPolicyFromString(t, `version: "1.0"
rules:
  - name: allow-lima-subnet
    priority: 10
    action: allow
    egress:
      domains: [subnet.lima.internal]
      protocols: [tcp]`)

	table, err := BuildFilterTable(pol, tracker, subnet, gatewayIP, false)
	assert.NilError(t, err)
	assert.Assert(t, len(table.Rules) > 0, "Expected rules in filter table")
}
