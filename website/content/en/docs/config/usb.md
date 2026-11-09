---
title: USB passthrough
weight: 95
---

> **Warning**
> `usb` and `limactl usb` are experimental

Lima passes host USB devices to a Linux guest over USB/IP on vsock.

Requires `vmType: vz` and `user.passwordlessSudo: true`. Ignored with `plain: true`.

```yaml
usb:
  # Allow `limactl usb attach`
  enabled: true
  # Import at boot
  devices:
  - vendorID: "0403"        # 4 hex digits, in quotes
    productID: "6015"
    name: "serial"          # optional, in quotes, shown by `limactl usb list`
    busAddr: "1-0x01100000" # optional BUSID, picks one of several identical devices
```

Off by default. `devices` alone implies `enabled: true`. An explicit `enabled: false` disables passthrough even with `devices` set.

`limactl validate` and `limactl start` validate this section.
`enabled` in `_config/override.yaml` wins over the instance, which wins over `_config/default.yaml`.
`devices` from all three are combined.

```bash
limactl usb list
limactl usb attach default 1-0x01100000
limactl usb detach default 1-0x01100000
```

`--vendor` with `--product` can replace the BUSID. `limactl usb list --json` prints one JSON object per line.

## Lifetime

- `devices` import at VM boot and at guest reboot.
- `limactl usb attach` is a runtime grant. It ends at unplug, at `limactl usb detach`, and at VM stop.
- A guest reboot keeps runtime grants, because the VM did not stop.
- `limactl usb detach` holds until VM stop, so a detached `devices` entry does not return at guest reboot. `limactl usb attach` lifts it.
- **Nothing reconnects a running guest.** A device that is replugged or re-enumerates stays detached until you attach it again. A matching `devices` entry imports it at the next guest boot.
  Firmware loaders and mode switches re-enumerate, so `dfu-util` and similar tools need a new `limactl usb attach`.
  For `rtw88` WiFi adapters, boot the guest with `rtw88_usb.switch_usb_mode=N`.
- One instance holds a device at a time. Other instances are refused, and the hostagent log names the holder.

## Caveats

- **No isochronous transfers.** These URBs fail with `-ESHUTDOWN`, so webcams and USB audio do not stream.
- **No hubs.** Hubs are not listed or exported. Devices behind them are.
- **macOS drivers.** An interface held by a macOS driver or a host process cannot be claimed.
  The hostagent logs `claim interface N: IOReturn 0x...`, and transfers on that interface fail.
- **USB storage.** On import, the hostagent unmounts the device's volumes on the host with `diskutil unmountDisk`.
  An open file on the host makes the import fail. macOS mounts the volumes again after detach.
  Unmount the filesystem in the guest before `limactl usb detach`.
- **Device held by another host client.** It keeps its current configuration, and a guest request for another one fails with `-EPIPE`.
- **Guest kernel.** The guest needs `vhci-hcd` and a driver for the device.
  On Ubuntu guests, Lima installs `linux-modules-extra-$(uname -r)` at boot when `vhci-hcd` is missing, as on Ubuntu 24.04,
  unless a `mode: dependency` provision script sets `skipDefaultDependencyResolution: true`.
  The stock Debian `genericcloud`, Fedora Cloud, and Alpine images have no `vhci-hcd`. They boot normally, but `limactl usb attach` fails until you install a kernel that has it.
- Each connection is limited to about 64 MiB of queued URBs. A guest over the limit loses the device.

The reason for a refused import is in `ha.stderr.log` of the instance.
A failed boot import leaves its reason in `/var/log/cloud-init-output.log` in the guest.
If the `vhci-hcd` install failed, for example with no network, restart the instance.

## Files

- `${LIMA_HOME}/_usbip-<busid>.lock` names the instance holding a host USB device.
- `usbip-allow.json` in the instance directory lists the USB devices the USB/IP server may export to that instance.
