# Network

## user-mode network (192.168.5.0/24)

By default Lima only enables the user-mode networking aka "slirp".

### Guest IP (192.168.5.15)

The guest IP address is set to `192.168.5.15`.

This IP address is not accessible from the host by design.

Use [vde_vmnet](https://github.com/lima-vm/vde_vmnet) to allow accessing the guest IP from the host and other guests.

### Host IP (192.168.5.2)

The loopback addresses of the host is `192.168.5.2` and is accessible from the guest as `host.lima.internal`.

### DNS (192.168.5.3)

The DNS.

If `useHostResolver` in `lima.yaml` is true, then the hostagent is going to run a DNS server over tcp and udp - each on a separate randomly selected free port. This server does a local lookup using the native host resolver, so it will deal correctly with VPN configurations and split-DNS setups, as well as mDNS, local `/etc/hosts` etc. For this the hostagent has to be compiled with `CGO_ENABLED=1` as default Go resolver is [broken](https://github.com/golang/go/issues/12524).

These tcp and udp ports are then forwarded via iptables rules to `192.168.5.3:53`, overriding the DNS provided by QEMU via slirp.

Currently following request types are supported:

- A
- AAAA
- CNAME
- TXT
- NS
- MX
- SRV

For all other queries hostagent will redirect the query to the nameservers specified in `/etc/resolv.conf` (or, if that fails - to `8.8.8.8` and `1.1.1.1`).

DNS over tcp is rarely used. It is usually only used either when user explicitly requires it, or when request+response can't fit into a single UDP packet (most likely in case of DNSSEC), or in the case of certain management operations such as domain transfers. Neither DNSSEC nor management operations are currently supported by a hostagent, but on the off chance that the response may contain an unusually long list of records - hostagent will also listen for the tcp traffic.

During initial cloud-init bootstrap, `iptables` may not yet be installed. In that case the repo server is determined using the slirp DNS. After `iptables` has been installed, the forwarding rule is applied, switching over to the hostagent DNS.

If `useHostResoler` is false, then DNS servers can be configured manually in `lima.yaml` via the `dns` setting. If that list is empty, then Lima will either use the slirp DNS (on Linux), or the nameservers from the `en0` host interface (on macOS).

## `vde_vmnet` (192.168.105.0/24)

[`vde_vmnet`](https://github.com/lima-vm/vde_vmnet) is required for adding another guest IP that is accessible from
the host and other guests.

To enable `vde_vmnet` (in addition the user-mode network), add the following lines to the YAML after installing `vde_vmnet`.

```yaml
networks:
  # vnl (virtual network locator) points to the vde_switch socket directory,
  # optionally with vde:// prefix
  # - vnl: "vde:///var/run/vde.ctl"
  #   # VDE Switch port number (not TCP/UDP port number). Set to 65535 for PTP mode.
  #   # Default: 0
  #   switchPort: 0
  #   # MAC address of the instance; lima will pick one based on the instance name,
  #   # so DHCP assigned ip addresses should remain constant over instance restarts.
  #   macAddress: ""
  #   # Interface name, defaults to "lima0", "lima1", etc.
  #   interface: ""
```

The IP address range is typically `192.168.105.0/24`, but depends on the configuration of `vde_vmnet`.
See [the documentation of `vde_vmnet`](https://github.com/lima-vm/vde_vmnet) for further information.

## Managed VMNet networks (via vde_vmnet)

Starting with version v0.7.0 lima can manage the networking daemons automatically. Networks are defined in
`$LIMA_HOME/_config/networks.yaml`. If this file doesn't already exist, it will be created with these default
settings:

```yaml
# Paths to vde executables. Because vde_vmnet is invoked via sudo it should be
# installed where only root can modify/replace it. This means also none of the
# parent directories should be writable by the user.
#
# The varRun directory also must not be writable by the user because it will
# include the vde_vmnet pid files. Those will be terminated via sudo, so replacing
# the pid files would allow killing of arbitrary privileged processes. varRun
# however MUST be writable by the daemon user.
#
# None of the paths segments may be symlinks, why it has to be /private/var
# instead of /var etc.
paths:
  vdeSwitch: /opt/vde/bin/vde_switch
  vdeVMNet: /opt/vde/bin/vde_vmnet
  varRun: /private/var/run/lima
  sudoers: /private/etc/sudoers.d/lima

group: staff

networks:
  shared:
    mode: shared
    gateway: 192.168.105.1
    dhcpEnd: 192.168.105.254
    netmask: 255.255.255.0
  bridged:
    mode: bridged
    interface: en0
    # bridged mode doesn't have a gateway; dhcp is managed by outside network
  host:
    mode: host
    gateway: 192.168.106.1
    dhcpEnd: 192.168.106.254
    netmask: 255.255.255.0
```

Instances can then reference these networks from their `lima.yaml` file:

```yaml
networks:
  # Lima can manage daemons for networks defined in $LIMA_HOME/_config/networks.yaml
  # automatically. Both vde_switch and vde_vmnet binaries must be installed into
  # secure locations only alterable by the "root" user.
  # - lima: shared
  #   # MAC address of the instance; lima will pick one based on the instance name,
  #   # so DHCP assigned ip addresses should remain constant over instance restarts.
  #   macAddress: ""
  #   # Interface name, defaults to "lima0", "lima1", etc.
  #   interface: ""
```

The network daemons are started automatically when the first instance referencing them is started,
and will stop automatically once the last instance has stopped. Daemon logs will be stored in the
`$LIMA_HOME/_networks` directory.

Since the commands to start and stop the `vde_vmnet` daemon requires root, the user either must
have password-less `sudo` enabled, or add the required commands to a `sudoers` file. This can
be done via:

```shell
limactl sudoers | sudo tee /etc/sudoers.d/lima
```
