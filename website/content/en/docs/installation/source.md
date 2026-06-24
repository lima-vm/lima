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

## Embedding Lima in a self-contained application

You can bundle this build of Lima inside another application (for example a macOS `.app`, or any
installer that ships its own copy) and drive it through the `limactl` CLI. Lima is integrated as a
CLI, not as a Go library, so the host application invokes `limactl` as a subprocess.

### What to bundle

`limactl` locates its support files **relative to its own executable**: for a binary at
`<prefix>/bin/limactl` it looks for `<prefix>/share/lima` (see `pkg/usrlocal`). Reproduce that
layout in your bundle:

```text
<your-app>/.../lima/
├── bin/
│   └── limactl                              # codesigned on macOS (see below)
└── share/lima/
    ├── lima-guestagent.Linux-aarch64.gz     # guest agent(s) for the guest arch(es) you run
    ├── lima-guestagent.Linux-x86_64.gz
    └── templates/                           # only if you use the built-in templates
```

`make PREFIX=<your-app>/.../lima install` (or `make clean native` and copying `_output/{bin,share}`)
produces exactly this tree. Ship only the guest-agent architectures you actually launch.

External runtime dependencies, by driver:

- **VZ (macOS host, Linux guest)** — nothing extra. `Virtualization.framework` is part of macOS, so a
  codesigned `limactl` is sufficient, including for [runtime hot-mount](../../config/mount#runtime-hot-mounts).
- **QEMU (Linux host)** — bundle or require `qemu-system-<arch>`, `virtiofsd` (for virtiofs and
  virtiofs hot-mount), and the UEFI firmware. Ensure they are on `PATH` (or pin absolute paths in your
  templates). The vendored `third_party/Code-Hex-vz` fork is **compile-time only** — it is linked into
  `limactl` and produces no runtime artifact to ship.

### Codesigning and entitlements (macOS)

The VZ driver requires `limactl` to carry the `com.apple.security.virtualization` entitlement (plus the
network entitlements in `vz.entitlements`). For a distributed, notarized app:

1. Sign the embedded `limactl` with your Developer ID, the hardened runtime, and `vz.entitlements`:
   ```bash
   codesign --force --options runtime --entitlements vz.entitlements \
     --sign "Developer ID Application: …" <your-app>/.../lima/bin/limactl
   ```
2. Sign the outer app, then **notarize the whole bundle**. `limactl` is a separate process with its own
   signature and entitlement, so the entitlement lives on the helper, not necessarily the parent app.
3. A plain `make` build signs `limactl` ad-hoc (`-s -`), which is fine for local use but not for
   distribution — re-sign with a Developer ID as above before notarizing.

### Runtime configuration

- Set `LIMA_HOME` to a writable, app-specific directory (e.g.
  `~/Library/Application Support/<YourApp>/lima`) so instance state does not collide with a user's own
  `~/.lima`.
- Make sure any external binaries (qemu, virtiofsd) are reachable via `PATH` from the environment in
  which you spawn `limactl`.
- Runtime hot-mount works unchanged when embedded; note that an instance must be **started by this
  build** to get the reserved hot-mount devices/slots.
