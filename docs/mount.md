# Filesystem mounts

Lima supports several methods for mounting the host filesystem into the guest.

The default mount type is shown in the following table:

| Lima Version     | Default                             |
| ---------------- | ----------------------------------- |
| < 0.10           | reverse-sshfs + Builtin SFTP server |
| >= 0.10          | reverse-sshfs + OpenSSH SFTP server |
| >= 1.0 (Planned) | 9p                                  |

## Which mount type should I pick? Or why care?

sshfs leverages mature networking tools to make a reliable filesystem mount.
Leaning on the classics.
9p implements innovative ideas from the experimental [Plan 9 operating system](https://9p.io/wiki/plan9/Overview/index.html).
Exploring new horizons.

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
The "9p" mount type is implemented by using QEMU's virtio-9p-pci devices.
virtio-9p-pci is also known as "virtfs", but note that this is unrelated to [virtio-fs](https://virtio-fs.gitlab.io/).

An example configuration:
```yaml
mountType: "9p"
mounts:
- location: "~"
  9p:
    # Supported security models are "passthrough", "mapped-xattr", "mapped-file" and "none".
    # 🟢 Builtin default: "mapped-xattr"
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
