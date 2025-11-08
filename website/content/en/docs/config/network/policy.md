---
title: Network Policy Filtering (user-v2)
weight: 35
---

| ⚡ Requirement | Lima >= 2.1, user-v2 network mode only |
|-------------------|----------------|

Network policy filtering for `user-v2` networks allows you to control egress (outbound) traffic using declarative policy rules.

## Overview

Provides:

- **Protocol filtering** (TCP, UDP, ICMP)
- **Port-based rules** (single ports or ranges)
- **IP/CIDR-based rules** (allow/deny specific destinations)
- **Domain-based rules** (with wildcard support like `*.example.com`)

## Configuration

### 1. Create a policy file

Policy files use YAML or JSON format. Example `~/my-policy.yaml`:

```yaml
version: "1.0"

rules:
  # Rules are evaluated in priority order (lowest priority number first)

  # Block cloud metadata service
  - name: block-cloud-metadata
    action: deny
    priority: 5
    egress:
      ips:
        - 169.254.169.254/32
        - fd00:ec2::254/128

  # Allow HTTPS to specific domains
  - name: allow-github
    action: allow
    priority: 10
    egress:
      protocols:
        - tcp
      domains:
        - github.com
        - "*.github.com"
        - "*.githubusercontent.com"
      ports:
        - "443"


  # Allow ICMP (ping)
  - name: allow-icmp
    action: allow
    priority: 25
    egress:
      protocols:
        - icmp

```

### 2. Create network with policy

Use `limactl network create` with the `--policy` flag:

```bash
limactl network create mynetwork --mode user-v2 --policy ~/my-policy.yaml
```

This validates and saves the policy for the network. The policy will be automatically applied when instances use this network.

### 3. Connect instances to the network

{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --network=lima:mynetwork
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
networks:
   - lima: mynetwork
```
{{% /tab %}}
{{< /tabpane >}}

## Policy Format

### Policy Structure

```yaml
version: "1.0"  # Required: policy version

rules:           # Required: list of filtering rules
  - name: rule-name         # Required: unique rule identifier
    action: allow|deny      # Required: "allow" or "deny"
    priority: <number>         # Required: evaluation priority (lower = higher priority)
    egress:                 # Optional: match criteria (omit to match all traffic)
      protocols:            # Optional: list of protocols
        - tcp|udp|icmp
      domains:              # Optional: list of domain patterns
        - example.com
        - "*.example.com"   # Wildcard support
      ips:                  # Optional: list of IP addresses or CIDR blocks
        - 192.168.1.0/24
        - 10.0.0.1
      ports:                # Optional: list of ports or port ranges
        - "443"
        - "8000-9000"
```

### Field Descriptions

- **version**: Policy format version (currently only `"1.0"` supported)
- **name**: Unique identifier for the rule
- **action**: `allow` (permit traffic) or `deny` (block traffic)
- **priority**: Numeric priority (rules evaluated from lowest to highest)
- **egress**: Match criteria for outbound traffic (all criteria are AND'ed together)
  - **protocols**: TCP, UDP, or ICMP
  - **domains**: Exact domains or wildcard patterns (requires DNS resolution)
  - **ips**: IP addresses or CIDR notation (IPv4 and IPv6 supported)
  - **ports**: Single ports (`"443"`) or ranges (`"8000-9000"`)

### Rule Evaluation

1. Rules are sorted by `priority` (lowest first)
2. For each packet, rules are evaluated in priority order
3. First matching rule determines the action (allow/deny)
4. If no rules match, traffic is **denied** by default
5. Domain-based rules match based on DNS query tracking with TTL

### Domain Matching

Domain-based rules use DNS query tracking:

- DNS responses are monitored and domain-to-IP mappings cached
- Wildcard patterns supported: `*.example.com` matches subdomains
- Mappings expire based on DNS TTL (with 10,000 domain limit)
- Both IPv4 (A) and IPv6 (AAAA) records are tracked

## Examples

### Example 1: Developer Workstation

Allow common development traffic, block everything else:

```yaml
version: "1.0"

rules:
  - name: allow-http-https
    action: allow
    priority: 10
    egress:
      protocols: [tcp]
      ports: ["80", "443"]

  - name: allow-ssh
    action: allow
    priority: 30
    egress:
      protocols: [tcp]
      ports: ["22"]
```

### Example 2: Restricted Environment

Allow only specific services:

```yaml
version: "1.0"

rules:
  - name: allow-package-repos
    action: allow
    priority: 10
    egress:
      protocols: [tcp]
      domains:
        - "*.ubuntu.com"
        - "*.debian.org"
        - "*.alpinelinux.org"
      ports: ["80", "443"]

```

### Example 3: Security-Focused

Block metadata services and limit egress:

```yaml
version: "1.0"

rules:
  - name: block-metadata
    action: deny
    priority: 1
    egress:
      ips:
        - 169.254.169.254/32  # AWS/GCP/Azure metadata
        - fd00:ec2::254/128    # AWS IPv6 metadata

  - name: allow-https-only
    action: allow
    priority: 20
    egress:
      protocols: [tcp]
      ports: ["443"]
```

## Troubleshooting

### Traffic being blocked unexpectedly

1. Check rule priority - ensure allow rules have lower priority numbers than deny rules
2. Verify all match criteria (protocols, ports, IPs/domains) are specified correctly
3. For domain-based rules, ensure DNS is allowed (UDP port 53)
4. Remember that all egress criteria within a rule are AND'ed together

### Domain-based rules not working

1. Ensure DNS traffic (UDP port 53) is allowed
2. Domain matching only works after DNS resolution occurs
3. Check that domain patterns are correct (`*.example.com` matches subdomains only)
4. DNS cache has a 10,000 domain limit; old entries are evicted

## Notes

- Policy filtering adds minimal overhead (DNS tracking and iptables rule evaluation)
- IPv6 is fully supported
- Policies are immutable after network start (restart network to apply changes)
- DNS tracking uses up to 10,000 entries with automatic expiration and eviction
