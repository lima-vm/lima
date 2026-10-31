---
title: user-v2 network
weight: 32
---

| ⚡ Requirement | Lima >= 0.16.0 |
|-------------------|----------------|

user-v2 network provides a user-mode networking similar to the [default user-mode network](#user-mode-network--1921685024-) and also provides support for `vm -> vm` communication.

To enable this network mode, define a network with `mode: user-v2` in networks.yaml

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

> **Note**
>
> Enabling user-v2 network will disable the [default user-mode network]({{< ref "/docs/config/network/user" >}}).

## Accessing VMs from the host

By default, the `lima-<NAME>.internal` hostnames are only resolvable from within the guest VMs.
To access the guest network from the host, use `limactl tunnel` to create a SOCKS proxy:

```bash
limactl tunnel <INSTANCE>
```

The command will output the proxy address (the port is randomly assigned):
```console
$ limactl tunnel default
Set `ALL_PROXY=socks5h://127.0.0.1:<PORT>`, etc.
The instance can be connected from the host as <http://lima-default.internal> via a web browser.
```

You can then access any VM on the user-v2 network from the host using the SOCKS proxy:

{{< tabpane text=true >}}
{{% tab header="curl" %}}
```bash
curl --proxy socks5h://127.0.0.1:<PORT> http://lima-default.internal
```
{{% /tab %}}
{{% tab header="Environment variable" %}}
```bash
export ALL_PROXY=socks5h://127.0.0.1:<PORT>
curl http://lima-default.internal
```
{{% /tab %}}
{{% tab header="Web browser" %}}
Configure your browser to use the SOCKS5 proxy at `127.0.0.1:<PORT>`,
then navigate to `http://lima-default.internal`.
{{% /tab %}}
{{< /tabpane >}}

> **Note**
>
> `limactl tunnel` is experimental.

## External daemon

| ⚡ Requirement | Lima >= 2.4 |
|-------------------|-------------|

> **Warning**
> `external: true` is [experimental]({{< ref "/docs/releases/experimental" >}}).

By default, the daemon of a user-v2 network is started by Lima with the first instance that uses the network,
and stopped with the last one.

To use a daemon that is not controlled by Lima, e.g., to filter the traffic of the guests,
set `external: true` in `networks.yaml`:

```yaml
networks:
  filtered:
    mode: user-v2
    external: true
    # Must match the subnet of the external daemon
    gateway: 192.168.110.1
    netmask: 255.255.255.0
```

Like for [unmanaged networks]({{< ref "/docs/config/network/unmanaged" >}}), the daemon must be started before the instance.
Lima never starts or stops the daemon, and an instance fails to start when the daemon is not running.

The daemon has to provide the same sockets as the daemon of Lima in `$LIMA_HOME/_networks/<NETWORK>/`.
