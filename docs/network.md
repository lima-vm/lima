# Network

## user-mode network (192.168.5.0/24)

By default Lima only enables the user-mode networking aka "slirp".

### Guest IP (192.168.5.15)

The guest IP address is typically set to 192.168.5.15.

This IP address is not accessible from the host by design.

Use `vde_vmnet` to allow accessing the guest IP from the host and other guests.

### Host IP (192.168.5.2)

The loopback addresses of the host is accessible from the guest as 192.168.5.2.

### DNS (192.168.5.3)

The DNS.

## `vde_vmnet` (192.168.105.0/24)

[`vde_vmnet`](https://github.com/lima-vm/vde_vmnet) is required for adding another guest IP that is accessible from
the host and other guests.

To enable `vde_vmnet` (in addition the user-mode network), add the following lines to the YAML after installing `vde_vmnet`.

```yaml
network:
  # The instance can get routable IP addresses from the vmnet framework using
  # https://github.com/lima-vm/vde_vmnet. Both vde_switch and vde_vmnet
  # daemons must be running before the instance is started. The interface type
  # (host, shared, or bridged) is configured in vde_vmnet and not lima.
  vde:
    # url points to the vde_switch socket directory, optionally with vde:// prefix
    - url: "vde:///var/run/vde.ctl"
      # MAC address of the instance; lima will pick one based on the instance name,
      # so DHCP assigned ip addresses should remain constant over instance restarts.
      macAddress: ""
      # Interface name, defaults to "vde0", "vde1", etc.
      name: ""
```

The IP address range is typically `192.168.105.0/24`, but depends on the configuration of `vde_vmnet`.
See [the documentation of `vde_vmnet`](https://github.com/lima-vm/vde_vmnet) for further information.
