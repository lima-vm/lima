---
title: Managed networks on Linux
weight: 34

---

| ⚡ Requirement    | Lima >= 2.2, Linux |
|-------------------|--------------------|

On Linux hosts, the `shared`, `host` and `bridged` networks defined in
`$LIMA_HOME/_config/networks.yaml` assign a "real" IP address that is reachable
from the host, in the same way [socket_vmnet]({{< ref "/docs/config/network/vmnet" >}})
does on macOS.

They are managed by `lima-net`, a privileged helper that ships with Lima and is
installed to `<PREFIX>/libexec/lima/lima-net`. It creates a Linux bridge per
network and attaches one tap device per instance.

## Requirements

`lima-net` calls the `dnsmasq` for DHCP and DNS, which must be installed.
Only QEMU instances are supported. The VZ and krunkit drivers are macOS-only.

## Setting up the `sudoers` file

Creating a bridge and a tap device requires root, so the user either must have
password-less `sudo` enabled, or add the required commands to a `sudoers` file:

```bash
limactl sudoers >etc_sudoers.d_lima
less etc_sudoers.d_lima  # verify that the file looks correct
sudo install -o root etc_sudoers.d_lima /etc/sudoers.d/lima
rm etc_sudoers.d_lima
```

The generated file whitelists the exact command lines Lima needs, and nothing
else:

```
%wheel ALL=(root:root) NOPASSWD:NOSETENV: \
    /usr/local/libexec/lima/lima-net start --pidfile=/run/lima/shared_lima-net.pid --mode=shared --bridge=lima-shared --gateway=192.168.105.1 --dhcp-end=192.168.105.254 --netmask=255.255.255.0, \
    /usr/bin/pkill -F /run/lima/shared_lima-net.pid, \
    /usr/local/libexec/lima/lima-net tap --bridge=lima-shared limatap[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]
```

Re-run `limactl sudoers` and re-install the file whenever `networks.yaml`
changes, otherwise `sudo` will reject the new command line.

The `group` setting in `networks.yaml` selects the group that is granted these
permissions; it defaults to `sudo` or `wheel`, whichever exists on the host.

`limactl sudoers` refuses to generate a file unless `paths.limaNet` is owned by
`root` and none of its parent directories are writable by the user, so Lima must
be installed with `sudo make install` rather than run from `_output`.

## Configuration

The defaults generated on a Linux host are:

<details>
<summary>Default</summary>

<p>

```yaml
paths:
  socketVMNet: ""
  limaNet: "/usr/local/libexec/lima/lima-net"
  varRun: /run/lima
  sudoers: /etc/sudoers.d/lima

group: wheel

networks:
  user-v2:
    mode: user-v2
    gateway: 192.168.104.1
    netmask: 255.255.255.0
  shared:
    mode: shared
    gateway: 192.168.105.1
    dhcpEnd: 192.168.105.254
    netmask: 255.255.255.0
  bridged:
    mode: bridged
    interface: br0
    # bridged mode doesn't have a gateway; dhcp is managed by outside network
  host:
    mode: host
    gateway: 192.168.106.1
    dhcpEnd: 192.168.106.254
    netmask: 255.255.255.0
```

</p>

</details>

Instances can then reference these networks:

{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --network=lima:shared
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
networks:
- lima: shared
  # MAC address of the instance; lima will pick one based on the instance name,
  # so DHCP assigned ip addresses should remain constant over instance restarts.
  macAddress: ""
  # Interface name, defaults to "lima0", "lima1", etc.
  interface: ""
```
{{% /tab %}}
{{< /tabpane >}}

The network daemon is started automatically when the first instance referencing
it is started, and stops automatically once the last instance has stopped.
Daemon logs are stored in the `$LIMA_HOME/_networks` directory.

## Modes

### shared (192.168.105.0/24)

Creates the `lima-shared` bridge. Guests reach the host, each other, and the
outside world through NAT (`MASQUERADE`, scoped to the network's own subnet).
Enabling this turns on `net.ipv4.ip_forward` on the host; Lima restores it on
teardown if it was the one that enabled it.

### host (192.168.106.0/24)

Creates the `lima-host` bridge. Guests reach the host and each other, but
`FORWARD` rules block everything else, including other Lima networks.

Note that an instance also has the default user-mode (slirp) network, which
still provides outbound connectivity. The `host` network only guarantees that no
traffic is routed *through the host* off the `lima-host` bridge.

### bridged

Attaches the instance directly to an existing host bridge, so that it gets an
address from the outside network's DHCP server and is reachable from the LAN.

Unlike on macOS, `interface` must name a **bridge that already exists**, created
by the host administrator with NetworkManager, systemd-networkd or netplan. Lima
never creates it and never enslaves a physical interface, because doing so can
disconnect the host.

> **Warning**
>
> A bridged instance is exposed to every host on the LAN, and its tap device
> stays on the bridge until the network is torn down.

## Firewall

Guests must be able to reach the host's DHCP and DNS ports, which host firewalls
block by default.

- With **firewalld**, the bridge is bound to the first existing zone out of
  `lima`, `nm-shared`, and `libvirt`. All three forward the guests' traffic but
  expose only DHCP, DNS and ICMP on the host itself. Define a `lima` zone to
  override that policy.
- With **ufw**, rules allowing UDP 67 and UDP/TCP 53 on the bridge are added.
- Otherwise, equivalent `iptables` rules are added.

All of them are removed when the network is torn down.

## Security

`lima-net` runs as root only to create the bridge and the tap devices. The tap
device is handed to the calling user (`ip tuntap add ... user "$UID"`), so QEMU
itself runs unprivileged and needs no network capabilities.

The `varRun` directory (`/run/lima`) must stay owned by root and must not be
writable by the user, because it holds the PID files that are passed to
`pkill -F` as root.
