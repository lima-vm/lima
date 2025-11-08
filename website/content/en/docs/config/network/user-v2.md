---
title: user-v2 network
weight: 32
---

| ⚡ Requirement | Lima >= 0.16.0 |
|-------------------|----------------|

user-v2 network provides a user-mode networking similar to the [default user-mode network](#user-mode-network--1921685024-) and also provides support for `vm -> vm` communication.

This network mode also supports [network policy filtering]({{< ref "/docs/config/network/policy" >}}) for egress traffic control.

To enable this network mode, define a network with `mode: user-v2` in networks.yaml or create one using `limactl network create`

By default, the below network configuration is already applied (Since v0.18).

```yaml
...
networks:
  user-v2:
    mode: user-v2
    gateway: 192.168.104.1
    netmask: 255.255.255.0
...
```

Instances can then reference these networks from their `lima.yaml` file:

{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --network=lima:user-v2
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
networks:
   - lima: user-v2
```
{{% /tab %}}
{{< /tabpane >}}

An instance's IP address is resolvable from another instance as `lima-<NAME>.internal.` (e.g., `lima-default.internal.`).

## Creating Networks

You can create custom user-v2 networks using the CLI:

```bash
# Basic network
limactl network create mynetwork --mode user-v2 --gateway 192.168.42.1/24

# Network with policy filtering
limactl network create secure-net --mode user-v2 --gateway 192.168.43.1/24 --policy ~/my-policy.yaml
```

The `--policy` flag allows you to specify a YAML or JSON policy file for [egress traffic filtering]({{< ref "/docs/config/network/policy" >}}).

_Note_

- Enabling this network will disable the [default user-mode network]({{< ref "/docs/config/network/user" >}})
- Policy filtering is only available for user-v2 networks
