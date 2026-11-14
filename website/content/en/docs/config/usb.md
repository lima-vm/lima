---
title: USB passthrough
weight: 95
---

> **Warning**
> USB passthrough is experimental, and requires the experimental [external vz driver]({{< ref "/docs/dev/drivers" >}}).

| ⚡ Requirement | Lima >= 2.4, macOS >= 27, vmType: vz |
| ------------- | ------------------------------------ |

Lima can attach USB devices connected to the host to a running instance.
The guest sees the devices on an XHCI controller.
A device attached to an instance cannot be used by the host or by other instances until it is detached.

## Requirements

USB passthrough uses Apple's AccessoryAccess framework, which requires:

- Lima built with the macOS 27 SDK or later.
- The vz driver signed with the `com.apple.developer.accessory-access.usb` entitlement.
  This is a restricted entitlement: it needs a provisioning profile from an Apple Developer account.
- An instance started from a terminal on the Mac. USB passthrough does not work for an instance started over SSH.

The Lima release binaries are ad-hoc signed, and do not support USB passthrough.
Without USB passthrough, an instance that specifies USB devices starts without them, with a warning.

## Build a signed vz driver

1. In the [Apple Developer portal](https://developer.apple.com/account/resources/identifiers/list),
   register an App ID (e.g., `com.example.lima-driver-vz`) with the "Claim USB Accessory" capability,
   and create a macOS App Development provisioning profile for it.

2. Build Lima with the vz driver as a signed app:

   ```bash
   make vz-usb-app \
     VZ_USB_PROVISIONING_PROFILE=~/Downloads/lima-driver-vz.provisionprofile \
     VZ_USB_SIGNING_IDENTITY="Apple Development: Example (TEAMID1234)"
   make install
   ```

   This wraps the driver in `libexec/lima/lima-driver-vz.app`, and replaces `libexec/lima/lima-driver-vz`
   with a symlink to it.

3. Check that the driver has the entitlement:

   ```bash
   codesign --display --entitlements - "$(dirname "$(which limactl)")/../libexec/lima/lima-driver-vz"
   ```

> **Note**
> A running instance keeps using the driver it started with. Restart it to use a rebuilt driver.

## Configuration

Devices that match an entry of `vmOpts.vz.usb.devices` are attached at boot,
and when they are connected while the instance is running.
Every field set in an entry must match.
`limactl start` waits up to 5 seconds for the devices.

An example configuration:
{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --set '.vmOpts.vz.usb.devices = [{"vendorID": 0x2cf0, "productID": 0x5250}]'
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
vmType: "vz"
vmOpts:
  vz:
    usb:
      devices:
      # A specific device
      - vendorID: 0x2cf0
        productID: 0x5250
        serial: "0123456789abcdef"
      # Any device from a vendor
      - vendorID: 0x0483
```
{{% /tab %}}
{{< /tabpane >}}

The IDs are shown by `limactl usb list`, or by System Information.

### Assigning devices to Lima

macOS lets Lima use only the devices assigned to it.
While an instance uses USB passthrough, the "Virtual Machine Accessories" item appears in the menu bar.
Choose "Use with" for the device, and select the terminal app that started the instance (e.g., Terminal).

macOS remembers the choice, so the device is attached whenever the instance starts.
Until a device is assigned, `limactl start` reports that no device matches the entry, and the instance starts without it.

If the menu bar item does not appear, check System Settings → Menu Bar.

## limactl usb

`limactl usb` attaches and detaches devices of a running instance, without changing its configuration.
It requires `vmOpts.vz.usb.devices`, or `vmOpts.vz.usb.enabled: true` to use it without attaching devices at boot.
With either setting, the "Virtual Machine Accessories" menu bar item appears when the instance starts:

```yaml
vmOpts:
  vz:
    usb:
      enabled: true
```

```console
$ limactl usb list default
ID             VENDOR:PRODUCT    ATTACHED    MANUFACTURER    PRODUCT        SERIAL
0x100005cab    2cf0:5250         true        Nuand           bladeRF 2.0    0123456789abcdef

$ limactl usb detach default 2cf0:5250

$ limactl usb attach default 2cf0:5250
```

A device is specified as `VENDOR:PRODUCT`, or by its ID in the first column.
The ID changes when the device is reconnected.

A device detached with `limactl usb detach` is not attached again for `vmOpts.vz.usb.devices`,
even when it is reconnected, until it is attached with `limactl usb attach`, or the instance restarts.
A device without a serial number is identified by the USB port it is connected to.

### Caveats
- Only devices assigned to Lima are listed and can be attached.
- Booting from a USB device requires it to be attached at boot. A device attached later is available to the running guest OS only.
- A device can be attached to only one instance at a time.
- Quitting the driver from the Dock is ignored. Stop the instance with `limactl stop`.
- Apple does not document which devices are supported.
- When the console user switches or logs out, macOS disconnects the devices, and reconnects them when the user returns.
