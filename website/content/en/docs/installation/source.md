---
title: Source Installation
weight: 30
---

## Installing from Source

If you prefer to build Lima from source, follow these steps:

### Prerequisites
Ensure you have the following dependencies installed:
- `git`
- `go`
- `make`
- On **macOS**: the Xcode Command Line Tools (`xcode-select --install`), required to build
  the VZ driver against `Virtualization.framework` (CGo).

### Build and Install
Run the following commands:

```bash
git clone https://github.com/lima-vm/lima
cd lima
make
sudo make install
```

> **Note:** `sudo make install` is required unless you have write permissions for `/usr/local`. Otherwise, installation may fail.

> **macOS / VZ:** `make` automatically codesigns `limactl` with the
> `com.apple.security.virtualization` entitlement (from `vz.entitlements`). This signature is
> required for the VZ driver, including runtime [hot-mount](../../config/mount#runtime-hot-mounts).
> If you re-sign or strip the binary, re-apply the entitlement with
> `codesign -f --entitlements vz.entitlements -s - <binary>`.

### Runtime hot-mount build note

[Runtime hot-mount](../../config/mount#runtime-hot-mounts) (`limactl mount add|remove|list`) needs
no extra build steps:

- **QEMU (Linux host):** built in by default.
- **VZ (macOS host):** depends on a small runtime addition to the `Code-Hex/vz` binding
  (`SetVirtioFileSystemDeviceShareAtIndex`). Until that is released upstream, the change is carried
  as an in-tree vendored fork under `third_party/Code-Hex-vz`, wired up via a `replace` directive in
  `go.mod`, so a plain `make` build is self-contained. The isolated change is also kept as
  `hack/patches/code-hex-vz-runtime-share.patch` for upstreaming; once it lands in a released
  `Code-Hex/vz`, drop `third_party/Code-Hex-vz`, remove the `replace` line, and bump the dependency.

### Alternative Installation (Without Sudo)
If you prefer installing Lima in your home directory, configure the `PREFIX` and `PATH` as follows:

```bash
make PREFIX=$HOME/.local install
export PATH=$HOME/.local/bin:$PATH
```

### Building External Drivers

> **⚠️ Building drivers as external mode is experimental**

Lima supports building drivers as external executables. For detailed information on creating and building external drivers, see the [Virtual Machine Drivers](../../dev/drivers) guide.

## Packaging Lima for Distribution
After building Lima from source, you may want to package it for installation on other machines:

1. The package for the core component and the native guest agent:
```bash
make clean native
cd _output
tar czf lima-package.tar.gz *
```

2. The package for the additional guest agents:
```
make clean additional-guestagents
cd _output
tar czf lima-additional-guestagents-package.tar.gz *
```

These packages can then be transferred and installed on the target system.
