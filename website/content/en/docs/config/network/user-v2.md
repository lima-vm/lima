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

A user-v2 network can be served by a program other than `limactl usernet`, for
example one that sends all guest traffic through a VPN. Set `daemon` to its
absolute path:

```yaml
networks:
  vpn:
    mode: user-v2
    gateway: 192.168.130.1
    netmask: 255.255.255.0
    daemon: /usr/local/libexec/vpn-usernet
```

Lima starts the daemon with the same flags as `limactl usernet` (`-p`, `-e`,
`--listen-qemu`, `--listen`, `--subnet`, and optionally `--leases`), and
expects the same HTTP API on the `-e` socket. `--subnet` is the network address
(`192.168.130.0/24` here), and the guest uses that address plus two
(`192.168.130.2`) as its gateway and DNS server.

Lima runs the daemon with your privileges, so like `paths.socketVMNet` it must
be a regular executable file that, together with every parent directory, is
owned by root and not writable by anyone else.

Because such a daemon decides all of the guest's network access, an instance
using it is rejected if it would give the guest another way out:

- the network must be the instance's only entry in `networks`;
- `portForwards` must not use `reverse: true`;
- no mount may contain, or be inside, `LIMA_HOME`, where the guest could read the
  daemon's state or rewrite `networks.yaml`. The default `~` mount is therefore
  refused; mount specific directories instead:

```yaml
networks:
  - lima: vpn
mounts:
  - location: "~/src"
    writable: true
```
