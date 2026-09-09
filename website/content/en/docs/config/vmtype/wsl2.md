---
title: WSL2
weight: 3
---

> **Warning**
> "wsl2" mode is experimental

| ⚡ Requirement | Lima >= 0.18 + (Windows >= 10 Build 19041 OR Windows 11) |
| ----------------- | -------------------------------------------------------- |

"wsl2" option makes use of native virtualization support provided by Windows' `wsl.exe` ([more info](https://learn.microsoft.com/en-us/windows/wsl/about)).

An example configuration:
{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --vm-type=wsl2 --mount-type=wsl2 --containerd=system
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
# Example to run Fedora using vmType: wsl2
vmType: wsl2
images:
# Source: https://github.com/runfinch/finch-core/blob/main/rootfs/Dockerfile
- location: "https://deps.runfinch.com/common/x86-64/finch-rootfs-production-amd64-1771357941.tar.gz"
  arch: "x86_64"
  digest: "sha256:423d1a0f1cabeaea6801995c90ed896dccc091180068626430f19fd87853fdf3"
mountType: wsl2
containerd:
  system: true
  user: false
```
{{% /tab %}}
{{< /tabpane >}}

### Caveats
- "wsl2" option is only supported on newer versions of Windows (roughly anything since 2019)

### Known Issues
- "wsl2" currently doesn't support many of Lima's options. See [this file](https://github.com/lima-vm/lima/blob/master/pkg/wsl2/wsl_driver_windows.go#L19) for the latest supported options.
- When running lima using "wsl2", `${LIMA_HOME}/<INSTANCE>/serial.log` will not contain kernel boot logs
- WSL2 requires a `tar` formatted rootfs archive instead of a VM image. Standard VM disk images (like `.qcow2`, `.raw`, etc.) or `.squashfs` images cannot be natively imported by WSL2.
- Lima unpacks a `.tar.gz` rootfs on its own, but a `.tar.xz`, `.tar.bz2`, or `.tar.zst` one needs the matching `xz`, `bzip2`, or `zstd` binary on your `PATH`, and Windows ships none of them.

### Rootfs Image Requirements & Building Custom Images

WSL2 does not run a standard virtual machine disk image directly. Instead, `wsl.exe` imports a guest root filesystem from a `.tar` or `.tar.gz` archive.

If you want to build and use your own custom rootfs, you can build it from a standard Linux container image using a Dockerfile:

1. **Create a Dockerfile:**
   Your custom rootfs must preinstall essential packages like `openssh-server`, `sudo`, `iptables`, and `sshfs`, and enable `user_allow_other` in `/etc/fuse.conf`. Here is an example using `ubuntu`:

   ```dockerfile
   FROM ubuntu:24.04

   # Install required dependencies
   RUN apt-get update && apt-get install -y --no-install-recommends \
       bash \
       openssh-server \
       sudo \
       iptables \
       sshfs \
       ca-certificates \
       && rm -rf /var/lib/apt/lists/*

   # Enable user_allow_other in fuse configuration
   RUN echo "user_allow_other" >> /etc/fuse.conf
   ```

2. **Build & Export the Rootfs Archive:**
   You can build the image and export its root filesystem directly as a `.tar` archive using Docker BuildKit's output option:
   ```bash
   docker build -o type=tar,dest=custom-rootfs.tar .
   ```

### Windows toolchain

Windows 10 and 11 ship an OpenSSH client in `C:\Windows\System32\OpenSSH\`, and
that's all the WSL2 driver needs.

The QEMU driver's reverse-sshfs mounts also want `sftp-server.exe`, which
belongs to OpenSSH Server, an [optional Feature on Demand](https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_install_firstuse).
Install it from an elevated PowerShell with
`Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0`. Without it
`sshocker` serves the mount in-process, so the mount still works. But pinning
`sftpDriver: openssh-sftp-server` under a mount's [`sshfs`]({{< ref "/docs/config/mount#reverse-sshfs" >}})
drops that fallback, and then the mount fails.

[Git for Windows](https://gitforwindows.org/) (`winget install -e --id Git.Git`)
is an alternative. Use the full installer; MinGit omits `scp.exe`,
`ssh-keygen.exe`, and `cygpath.exe`. Add `C:\Program Files\Git\usr\bin\` to your
`PATH` ahead of any other directory holding an `ssh.exe`; the installer's
default adds only `cmd\`, which holds none of these binaries. `usr\bin` also
carries `find.exe` and `sort.exe`, which shadow the Windows commands of those
names.

Order matters because Lima takes the first directory on `PATH` that holds
`ssh.exe`, `scp.exe`, and `ssh-keygen.exe` together, and falls back to
`System32\OpenSSH`. Lima then takes the `sftp-server` from that toolchain, and
writes the key and socket paths it hands `ssh` in the form that toolchain
expects. MSYS2 and Git for Windows get `cygpath`'s `/c/Users/USER`, stock
Cygwin `/cygdrive/c/Users/USER`, and the native client `C:/Users/USER`.

The [`SSH`]({{< ref "/docs/config/environment-variables#ssh" >}}) environment
variable overrides that choice, but parts of Lima still follow `PATH`, so point
both at the same install. A Cygwin toolchain in `SSH` with native OpenSSH
leading `PATH` fails `limactl start`.

A mount without an explicit `mountPoint` derives its guest path from `cygpath`
on every load, so adding stock Cygwin to `PATH` can move an existing instance's
mount inside the guest. Set `mountPoint` to pin it.
