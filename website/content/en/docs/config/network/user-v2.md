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

`gateway` and `netmask` are required, and must match the subnet of the daemon.
The guest uses the network address plus two (e.g., `192.168.110.2`) as its gateway and DNS server,
and the daemon has to give the guest a DHCP lease for the MAC address of the instance.

The daemon has to serve the same sockets as `limactl usernet` in `$LIMA_HOME/_networks/<NETWORK>/`:

- `<NETWORK>_ep.sock`: the HTTP API of [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock),
  required for every VM type. Lima checks that `GET /services/dhcp/leases` responds before starting the instance.
- `<NETWORK>_fd.sock`: used by the `vz` VM type. Lima connects and sends a file descriptor of a datagram socket
  pair, on which Ethernet frames are exchanged (the `bess` protocol of gvisor-tap-vsock).
- `<NETWORK>_qemu.sock`: used by the `qemu` VM type (the `qemu` protocol of gvisor-tap-vsock).

Only the socket used by the VM type of the instance is needed. The instance fails to start when it is missing.
See `pkg/networks/usernet/gvproxy.go` for the reference implementation.

Stop all the instances that use the network before changing `external`.

Limitations:

- When the daemon is restarted, the instances that are running lose their network until they are restarted.
- On `vz` with Linux guests, the SSH connection of Lima uses vsock by default, which does not go through the daemon.
  Set `ssh.overVsock: false` in the instance configuration to route it through the daemon.
- Port forwards are tunneled through the SSH or guest agent connection, so the daemon cannot filter them.
