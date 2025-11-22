// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Policy represents the complete network policy.
type Policy struct {
	Version string       `yaml:"version" json:"version"`
	Rules   []PolicyRule `yaml:"rules" json:"rules"`
}

// PolicyRule represents a single filtering rule.
type PolicyRule struct {
	Name     string       `yaml:"name" json:"name"`
	Action   string       `yaml:"action" json:"action"` // "allow" or "deny"
	Priority int          `yaml:"priority" json:"priority"`
	Egress   *PolicyMatch `yaml:"egress,omitempty" json:"egress,omitempty"` // nil = match all traffic
}

// PolicyMatch specifies what traffic the rule matches.
type PolicyMatch struct {
	Protocols []string `yaml:"protocols,omitempty" json:"protocols,omitempty"` // tcp, udp, icmp
	Domains   []string `yaml:"domains,omitempty" json:"domains,omitempty"`     // supports wildcards (*.example.com)
	IPs       []string `yaml:"ips,omitempty" json:"ips,omitempty"`             // supports CIDR notation
	Ports     []string `yaml:"ports,omitempty" json:"ports,omitempty"`         // single ports or ranges (8000-9000)
}

// IsAllowRule returns true if the rule is an allow rule.
func (r *PolicyRule) IsAllowRule() bool {
	return r.Action == "allow"
}

// IsDenyRule returns true if the rule is a deny rule.
func (r *PolicyRule) IsDenyRule() bool {
	return r.Action == "deny"
}

// MatchesAll returns true if the rule matches all traffic (egress is nil or all fields empty).
func (r *PolicyRule) MatchesAll() bool {
	if r.Egress == nil {
		return true
	}
	return len(r.Egress.Protocols) == 0 &&
		len(r.Egress.Domains) == 0 &&
		len(r.Egress.IPs) == 0 &&
		len(r.Egress.Ports) == 0
}

// LoadPolicy loads and parses an egress policy from a YAML or JSON file.
// Automatically detects format based on file extension.
func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file: %w", err)
	}

	var policy Policy
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &policy); err != nil {
			return nil, fmt.Errorf("failed to parse policy JSON: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &policy); err != nil {
			return nil, fmt.Errorf("failed to parse policy YAML: %w", err)
		}
	default:
		// Try YAML first, then JSON
		if err := yaml.Unmarshal(data, &policy); err != nil {
			if jsonErr := json.Unmarshal(data, &policy); jsonErr != nil {
				return nil, fmt.Errorf("failed to parse policy as YAML: %w, or JSON: %w", err, jsonErr)
			}
		}
	}

	if err := validatePolicy(&policy); err != nil {
		return nil, fmt.Errorf("invalid policy: %w", err)
	}

	// Sort rules by priority (lower number = higher priority)
	sort.Slice(policy.Rules, func(i, j int) bool {
		return policy.Rules[i].Priority < policy.Rules[j].Priority
	})

	return &policy, nil
}

// SavePolicyJSON saves a policy to a JSON file with nice formatting.
func SavePolicyJSON(policy *Policy, path string) error {
	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal policy to JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write policy file: %w", err)
	}

	return nil
}

// validatePolicy checks if the policy is valid.
func validatePolicy(policy *Policy) error {
	if policy.Version == "" {
		return errors.New("version field is required")
	}

	// Only version 1.0 is currently supported
	if policy.Version != "1.0" {
		return fmt.Errorf("unsupported policy version: %s (only '1.0' is supported)", policy.Version)
	}

	if len(policy.Rules) == 0 {
		return errors.New("at least one rule is required")
	}

	ruleNames := make(map[string]bool)
	for i, rule := range policy.Rules {
		// Check for unique rule names
		if rule.Name == "" {
			return fmt.Errorf("rule at index %d: name is required", i)
		}
		if ruleNames[rule.Name] {
			return fmt.Errorf("duplicate rule name: %s", rule.Name)
		}
		ruleNames[rule.Name] = true

		// Validate action
		if rule.Action != "allow" && rule.Action != "deny" {
			return fmt.Errorf("rule '%s': action must be 'allow' or 'deny', got '%s'", rule.Name, rule.Action)
		}

		// Validate egress match criteria
		// Note: Domain-based deny rule validation deferred to BuildFilterTable (needs DNS tracker)
		if rule.Egress != nil {
			// Validate protocols
			for _, proto := range rule.Egress.Protocols {
				if proto != "tcp" && proto != "udp" && proto != "icmp" {
					return fmt.Errorf("rule '%s': invalid protocol '%s' (must be tcp, udp, or icmp)", rule.Name, proto)
				}
			}

			// Validate ports
			for _, portStr := range rule.Egress.Ports {
				if err := validatePortRange(portStr); err != nil {
					return fmt.Errorf("rule '%s': invalid port specification '%s': %w", rule.Name, portStr, err)
				}
			}

			// Validate IPs/CIDRs
			for _, ipStr := range rule.Egress.IPs {
				if err := validateIPOrCIDR(ipStr); err != nil {
					return fmt.Errorf("rule '%s': invalid IP or CIDR '%s': %w", rule.Name, ipStr, err)
				}
			}
		}
	}

	return nil
}

// validatePortRange validates a port string (single port or range).
func validatePortRange(portStr string) error {
	if strings.Contains(portStr, "-") {
		parts := strings.Split(portStr, "-")
		if len(parts) != 2 {
			return errors.New("invalid port range format")
		}
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid start port: %w", err)
		}
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid end port: %w", err)
		}
		if start < 1 || start > 65535 {
			return fmt.Errorf("start port %d out of range (1-65535)", start)
		}
		if end < 1 || end > 65535 {
			return fmt.Errorf("end port %d out of range (1-65535)", end)
		}
		if start > end {
			return fmt.Errorf("start port %d greater than end port %d", start, end)
		}
	} else {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("invalid port: %w", err)
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("port %d out of range (1-65535)", port)
		}
	}
	return nil
}

// validateIPOrCIDR validates an IP address or CIDR notation.
func validateIPOrCIDR(ipStr string) error {
	// Try parsing as CIDR first
	_, _, err := net.ParseCIDR(ipStr)
	if err == nil {
		return nil
	}

	// Try parsing as IP
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return errors.New("not a valid IP address or CIDR notation")
	}

	return nil
}
