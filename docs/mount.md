# Filesystem mounts

Lima supports several methods for mounting the host filesystem into the guest.

The default mount type is shown in the following table:

| Lima Version     | Default                             |
| ---------------- | ----------------------------------- |
| < 0.10           | reverse-sshfs + Builtin SFTP server |
| >= 0.10          | reverse-sshfs + OpenSSH SFTP server |
| >= 1.0 (Planned) | 9p for QEMU, virtiofs for VZ        |

## Mount types

### reverse-sshfs
The "reverse-sshfs" mount type exposes the host filesystem by running an SFTP server on the host.
While the host works as an SFTP server, the host does not open any TCP port,
as the host initiates an SSH connection into the guest and let the guest connect to the SFTP server via the stdin.

An example configuration:
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
```yaml
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

The "9p" mount type requires Lima v0.10.0 or later.

#### Caveats
- The "9p" mount type is known to be incompatible with CentOS, Rocky Linux, and AlmaLinux as their kernel do not support `CONFIG_NET_9P_VIRTIO`.

### virtiofs
> **Warning**
> "virtiofs" mode is experimental

| :zap: Requirement | Lima >= 0.14, macOS >= 13.0 |
|-------------------|-----------------------------|

The "virtiofs" mount type is implemented by using apple Virtualization.Framework shared directory (uses virtio-fs) device. 
Linux guest kernel must enable the CONFIG_VIRTIO_FS support for this support.

An example configuration:
```yaml
vmType: "vz"
mountType: "virtiofs"
mounts:
  - location: "~"
```

#### Caveats
- The "virtiofs" mount type is supported only on macOS 13 or above with `vmType: vz` config. See also [`vmtype.md`](./vmtype.md).
