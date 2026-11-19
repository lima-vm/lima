---
title: USB passthrough design
weight: 40
---

This page explains why [USB passthrough]({{< ref "/docs/config/usb" >}}) uses USB/IP.

## USB/IP

The Linux guest kernel already ships the USB/IP client, `vhci-hcd`.
Lima adds a USB/IP server in the hostagent and an attach step in `lima-guestagent`.
The protocol needs only a byte stream between host and guest.
That is why the config is a top-level `usb` key, although only the `vz` driver implements it today.

## Native passthrough

macOS 27 adds `VZUSBPassthroughDeviceConfiguration`, which takes an `AAUSBAccessory` from the AccessoryAccess framework.
[`AAUSBAccessoryManager`](https://developer.apple.com/documentation/accessoryaccess/aausbaccessorymanager) presents UI for the app,
so it only works from an app that appears in the Dock and is signed with the restricted `com.apple.developer.accessory-access.usb` entitlement.
Restricted entitlements need a [provisioning profile](https://developer.apple.com/documentation/technotes/tn3125-inside-code-signing-provisioning-profiles),
which a standalone executable cannot carry. `limactl` is ad-hoc signed (`codesign -s -` with `vz.entitlements`).

Lima would have to ship a separately signed helper app that runs in the user's GUI session.
Builds from source and Homebrew builds would not get it, and it would need macOS 27 or later.
See [#2224](https://github.com/lima-vm/lima/issues/2224).

## Existing servers

- The only Go USB/IP server that exports real macOS devices, SagerNet/sing-usbip, is GPL, and its README says macOS export needs SIP disabled.
- The permissively licensed Go USB/IP libraries either hand the socket to the Linux kernel's `usbip-host` driver or serve emulated devices.
- Other macOS USB/IP servers, jiegec/usbip (Rust) and usbipd-mac (Swift), are standalone daemons on plain TCP port 3240.
- libusb, and gousb, its Go wrapper, only cover device access, so they would replace only the IOKit layer.
  libusb is LGPL-2.1, which is not on the CNCF allowed-license list.
  It can only take a device from a macOS driver as root or with the `com.apple.vm.device-access` entitlement.

## Code layout

- `pkg/usbip/server.go` and `pkg/usbip/protocol.go` speak the protocol: the device list, import, relaying each URB, unlink,
  and ending a session without leaving the guest's URBs unanswered.
  They reach hardware only through the `Device` and `Provider` interfaces in `pkg/usbip/device.go`.
- `pkg/usbip/device_iokit_darwin.go` is the IOKit backend, and the only file that touches hardware.
  On the guest's `SET_CONFIGURATION` it reconfigures the device with interface matching off, through the IOUSBHost seize option,
  so macOS drivers such as mass storage cannot take the interfaces back. This needs no root.
- Guest side: `pkg/usbip/attach_linux.go` and `pkg/usbip/vhci.go`.

## Transport and trust

The server listens only on Virtualization.framework vsock port 2223 of its VM.
No TCP or unix socket is opened on the host.
USB/IP has no authentication, so it is not exposed on TCP.
This is the same model as the guest agent on vsock port 2222.

The host decides what is exported: `usbip-allow.json` (mode 0600) in the instance directory,
written only by `limactl` and the hostagent, plus a per-device `flock` lock so one instance holds a device at a time.
Inside the guest, any process that can open an `AF_VSOCK` socket, not only root, can import an allowlisted device.

## Not supported

Isochronous transfers, hubs, and interfaces held by a macOS driver or a host process.
See [Caveats]({{< ref "/docs/config/usb#caveats" >}}).
