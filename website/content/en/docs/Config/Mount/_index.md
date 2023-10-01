---
title: Filesystem mounts
weight: 50
---

Lima supports several methods for mounting the host filesystem into the guest.

The default mount type is shown in the following table:

| Lima Version     | Default                                                       |
| ---------------- | ------------------------------------------------------------- |
| < 0.10           | reverse-sshfs + Builtin SFTP server                           |
| >= 0.10          | reverse-sshfs + OpenSSH SFTP server                           |
| >= 0.17          | reverse-sshfs + OpenSSH SFTP server for QEMU, virtiofs for VZ |
| >= 1.0 (Planned) | 9p for QEMU, virtiofs for VZ                                  |

## Mount types

### reverse-sshfs
The "reverse-sshfs" mount type exposes the host filesystem by running an SFTP server on the host.
While the host works as an SFTP server, the host does not open any TCP port,
as the host initiates an SSH connection into the guest and let the guest connect to the SFTP server via the stdin.

An example configuration:
{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --mount-type=reverse-sshfs
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
mountType: "reverse-sshfs"
mounts:
- location: "~"
  sshfs:
    # Enabling the SSHFS cache will increase performance of the mounted filesystem, at
    # the cost of potentially not reflecting changes made on the host in a timely manner.
    # Warning: It looks like PHP filesystem access does not work correctly when
    # the cache is disabled.
    # 🟢 Builtin default: true
    cache: null
    # SSHFS has an optional flag called 'follow_symlinks'. This allows mounts
    # to be properly resolved in the guest os and allow for access to the
    # contents of the symlink. As a result, symlinked files & folders on the Host
    # system will look and feel like regular files directories in the Guest OS.
    # 🟢 Builtin default: false
    followSymlinks: null
    # SFTP driver, "builtin" or "openssh-sftp-server". "openssh-sftp-server" is recommended.
    # 🟢 Builtin default: "openssh-sftp-server" if OpenSSH SFTP Server binary is found, otherwise "builtin"
    sftpDriver: null
```
{{% /tab %}}
{{< /tabpane >}}

The default value of `sftpDriver` has been set to "openssh-sftp-server" since Lima v0.10, when an OpenSSH SFTP Server binary
such as `/usr/libexec/sftp-server` is detected on the host.
Lima prior to v0.10 had used "builtin" as the SFTP driver.

#### Caveats
- A mount is disabled when the SSH connection was shut down.
- A compromised `sshfs` process in the guest may have an access to unexposed host directories.

### 9p
> **Warning**
> "9p" mode is experimental

The "9p" mount type is implemented by using QEMU's virtio-9p-pci devices.
virtio-9p-pci is also known as "virtfs", but note that this is unrelated to [virtio-fs](https://virtio-fs.gitlab.io/).

An example configuration:
{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --vm-type=qemu --mount-type=9p
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
vmType: "qemu"
mountType: "9p"
mounts:
- location: "~"
  9p:
    # Supported security models are "passthrough", "mapped-xattr", "mapped-file" and "none".
    # "mapped-xattr" and "mapped-file" are useful for persistent chown but incompatible with symlinks.
    # 🟢 Builtin default: "none" (since Lima v0.13)
    securityModel: null
    # Select 9P protocol version. Valid options are: "9p2000" (legacy), "9p2000.u", "9p2000.L".
    # 🟢 Builtin default: "9p2000.L"
    protocolVersion: null
    # The number of bytes to use for 9p packet payload, where 4KiB is the absolute minimum.
    # 🟢 Builtin default: "128KiB"
    msize: null
    # Specifies a caching policy. Valid options are: "none", "loose", "fscache" and "mmap".
    # Try choosing "mmap" or "none" if you see a stability issue with the default "fscache".
    # See https://www.kernel.org/doc/Documentation/filesystems/9p.txt
    # 🟢 Builtin default: "fscache" for non-writable mounts, "mmap" for writable mounts
    cache: null
```
{{% /tab %}}
{{< /tabpane >}}

The "9p" mount type requires Lima v0.10.0 or later.

#### Caveats
- The "9p" mount type is known to be incompatible with CentOS, Rocky Linux, and AlmaLinux as their kernel do not support `CONFIG_NET_9P_VIRTIO`.

### virtiofs
> **Warning**
> "virtiofs" mode is experimental

| ⚡ Requirement | Lima >= 0.14, macOS >= 13.0 | Lima >= 0.17.0, Linux, QEMU 4.2.0+, virtiofsd (Rust version) |
|-------------------|-----------------------------| ------------------------------------------------------------ |

The "virtiofs" mount type is implemented via the virtio-fs device by using apple Virtualization.Framework shared directory on macOS and virtiofsd on Linux.
Linux guest kernel must enable the CONFIG_VIRTIO_FS support for this support.

An example configuration:
{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --vm-type=vz --mount-type=virtiofs
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
vmType: "vz"  # only for macOS; Linux uses 'qemu'
mountType: "virtiofs"
mounts:
- location: "~"
```
{{% /tab %}}
{{< /tabpane >}}

#### Caveats
- For macOS, the "virtiofs" mount type is supported only on macOS 13 or above with `vmType: vz` config. See also [`vmtype.md`](./vmtype.md).
- For Linux, the "virtiofs" mount type requires the [Rust version of virtiofsd](https://gitlab.com/virtio-fs/virtiofsd).
  Using the version from QEMU (usually packaged as `qemu-virtiofsd`) will *not* work, as it requires root access to run.

<!-- WSL2 driver seems currently unstable -->
<!--
### wsl2
> **Warning**
> "wsl2" mode is experimental

| ⚡ Requirement | Lima >= 0.18 + (Windows >= 10 Build 19041 OR Windows 11) |
| ----------------- | -------------------------------------------------------- |

The "wsl2" mount type relies on using WSL2's navite disk sharing, where the root disk is available by default at `/mnt/$DISK_LETTER` (e.g. `/mnt/c/`).

An example configuration:
{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --vm-type=wsl2 --mount-type=wsl2
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
vmType: "wsl2"
mountType: "wsl2"
```
{{% /tab %}}
{{< /tabpane >}}

#### Caveats
- WSL2 file permissions may not work exactly as expected when accessing files that are natively on the Windows disk ([more info](https://github.com/MicrosoftDocs/WSL/blob/mattw-wsl2-explainer/WSL/file-permissions.md))
- WSL2's disk sharing system uses a 9P protocol server, making the performance similar to [Lima's 9p](#9p) mode ([more info](https://github.com/MicrosoftDocs/WSL/blob/mattw-wsl2-explainer/WSL/wsl2-architecture.md#wsl-2-architectural-flow))
-->
