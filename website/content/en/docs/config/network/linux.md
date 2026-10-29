---
title: Managed networks on Linux
weight: 34

---

| ⚡ Requirement    | Lima >= 2.3, Linux |
|-------------------|--------------------|

> **Warning**
>
> This feature is [experimental]({{< ref "/docs/releases/experimental" >}}) and subject to change.

On Linux hosts, the `shared`, `host` and `bridged` networks defined in
`$LIMA_HOME/_config/networks.yaml` assign a "real" IP address that is reachable
from the host, in the same way [socket_vmnet]({{< ref "/docs/config/network/vmnet" >}})
does on macOS.

They are managed by `lima-privileged-net`, a privileged helper that ships with
Lima and is installed to `<PREFIX>/libexec/lima/privileged/lima-privileged-net`.
It creates a Linux bridge per network and attaches one tap device per instance.

## Requirements

`lima-privileged-net` calls the `dnsmasq` for DHCP and DNS, which must be installed.
Only QEMU instances are supported. The VZ and krunkit drivers are macOS-only.

## Setting up the `sudoers` file

Creating a bridge and a tap device requires root, so the user either must have
password-less `sudo` enabled, or add the required commands to a `sudoers` file:

```bash
limactl sudoers >etc_sudoers.d_lima
less etc_sudoers.d_lima  # verify that the file looks correct
sudo install -o root -m 0440 etc_sudoers.d_lima /etc/sudoers.d/lima
rm etc_sudoers.d_lima
```

The generated file whitelists the exact command lines Lima needs, and nothing
else:

```
# Manage "shared" network daemons

%wheel ALL=(root:root) NOPASSWD:NOSETENV: \
    sha256:f705f79886fc248802e0bc388a66e2a61d09c827471d9639a5e558cf71080c72 /usr/local/libexec/lima/privileged/lima-privileged-net start --pidfile=/run/lima/shared_lima-privileged-net.pid --mode=shared --bridge=lima-shared --gateway=192.168.105.1 --dhcp-end=192.168.105.254 --netmask=255.255.255.0, \
    sha256:f705f79886fc248802e0bc388a66e2a61d09c827471d9639a5e558cf71080c72 /usr/local/libexec/lima/privileged/lima-privileged-net tap --bridge=lima-shared --network=shared -- *, \
    sha256:f705f79886fc248802e0bc388a66e2a61d09c827471d9639a5e558cf71080c72 /usr/local/libexec/lima/privileged/lima-privileged-net stop --pidfile=/run/lima/shared_lima-privileged-net.pid
```

The instance ID, a hash of the instance directory, is the only wildcard in the
`tap` line. The `--` ends the helper's flag parsing, so an ID that smuggles in
extra words - including a second `--bridge` - is rejected as a surplus
positional argument. The device name itself is never passed in: the helper
derives it from the network, the instance, and the uid that `sudo` reports, so
a tap device can only ever belong to the user who asked for it.

The `sha256:` prefix is a sudo [`Digest_Spec`](https://www.sudo.ws/docs/man/sudoers.man/#Digest_Spec)
(sudo >= 1.8.7): sudo hashes the helper right before executing it and falls back
to asking for a password when the binary no longer matches, so a helper that was
swapped out after the file was installed cannot be run as root. Unlike the
ownership checks `limactl` performs, this is enforced inside `sudo` itself and is
therefore not subject to a time-of-check/time-of-use race.

Re-run `limactl sudoers` and re-install the file whenever `networks.yaml` changes
or `lima-privileged-net` is reinstalled, otherwise `sudo` will reject the command
line. As the installed file is not readable by the user, check it with
`sudo cat /etc/sudoers.d/lima | limactl sudoers --check /dev/stdin`, which
reports when it is out of sync.

The `group` setting in `networks.yaml` selects the group that is granted these
permissions; it defaults to `sudo` or `wheel`, whichever exists on the host.

`limactl sudoers` refuses to generate a file unless the helper is owned by `root`
and none of its parent directories are writable by the user, so Lima must be
installed with `sudo make install` rather than run from `_output`.

The path of the helper is not configurable: it is always
`<PREFIX>/libexec/lima/privileged/lima-privileged-net`, resolved relative to the
`limactl` binary that is running.

## Configuration

`networks.yaml` is generated once and never rewritten. Earlier Lima releases
wrote the macOS defaults into it on Linux too; Lima rejects such a file because
of its `paths.socketVMNet`, so delete `~/.lima/_config/networks.yaml` to
regenerate it.

The defaults generated on a Linux host are:

<details>
<summary>Default</summary>

<p>

```yaml
paths:
  socketVMNet: ""
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
Enabling this turns on `net.ipv4.ip_forward` on the host. If Lima was the one
that enabled it, Lima turns it off again on teardown, unless another Lima
network is still up at that point; in that case it stays on.

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

For example, with NetworkManager, the following creates a `br0` bridge and moves
the host's address from the physical interface `eth0` onto it (replace `eth0` with
the name shown by `nmcli device`). Run it from a local console, as the host loses
its connection while the address moves to the bridge:

```bash
sudo nmcli connection add type bridge ifname br0 con-name br0 bridge.stp no
sudo nmcli connection add type bridge-slave ifname eth0 master br0 con-name br0-port-eth0
sudo nmcli connection up br0-port-eth0
```

The bridge then obtains its address from the LAN's DHCP server. Set `interface`
of the `bridged` network in `networks.yaml` to the bridge name if it is not `br0`.
Wi-Fi interfaces usually cannot be enslaved to a bridge.

> **Warning**
>
> A bridged instance is exposed to every host on the LAN, and its tap device
> stays on the bridge until the network is torn down.

## Firewall

Guests must be able to reach the host's DHCP and DNS ports, which host firewalls
block by default.

- With **firewalld**, the bridge is bound to the first existing zone out of
  `lima`, `nm-shared` (installed by NetworkManager), and `libvirt` (installed by
  libvirt). All three forward the guests' traffic and block most services on the
  host itself, but they do not expose the same ones:
  - `lima`, which Lima creates permanently when none of the three exists, exposes
    only DHCP, DNS and ICMP.
  - `nm-shared` also exposes SSH.
  - `libvirt` also exposes SSH, TFTP and DHCPv6.

  Define a `lima` zone to override that policy.
- With **ufw**, rules allowing UDP 67 and UDP/TCP 53 on the bridge are added.
- Otherwise, equivalent `iptables` rules are added.

The bridge is removed from the zone, and the ufw and `iptables` rules are
deleted, when the network is torn down. A `lima` zone that Lima created stays.

## Security

`lima-privileged-net` runs as root for as long as the network is up. It creates
the bridge and the tap devices, configures the firewall and forwarding with
`firewall-cmd`, `ufw`, `iptables` and `sysctl`, and runs `dnsmasq` as a root
child process. The tap device is handed to the calling user (`ip tuntap add ... user "$UID"`), so
QEMU itself runs unprivileged and needs no network capabilities.

The `varRun` directory (`/run/lima`) must stay owned by root and must not be
writable by the user, because it holds the PID files that `lima-privileged-net stop`
reads as root to pick the process to signal.
