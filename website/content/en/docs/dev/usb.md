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

## Host privileges

USB/IP needs no root on the host, no privileged helper, and no entitlement beyond `vz.entitlements`.
`limactl` and the hostagent run as the user, and every host-side step works without root:

- Listing devices and opening them through IOKit.
- Reconfiguring a device with interface matching off, through the IOUSBHost seize option.
- `diskutil unmountDisk` for the volumes of USB storage.
- The per-device `flock` lock files in `${LIMA_HOME}`.

The cost of staying unprivileged: an interface held by a macOS driver or by another host process cannot be claimed.
See [Caveats]({{< ref "/docs/config/usb#caveats" >}}).

Root is used only inside the guest, to attach the device to `vhci-hcd`.

## Why not use native passthrough

macOS 27 adds `VZUSBPassthroughDeviceConfiguration`, but it needs the restricted `com.apple.developer.accessory-access.usb` entitlement.
The process running the VM would have to be an app bundle signed with an Apple Developer account and a provisioning profile,
as in [#5790](https://github.com/lima-vm/lima/pull/5790). Lima releases are ad-hoc signed. USB/IP needs no signed app.
See [#2224](https://github.com/lima-vm/lima/issues/2224).

## Why not use existing servers

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
