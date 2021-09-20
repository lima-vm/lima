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
