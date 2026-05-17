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

Lima uses an OpenSSH installation on the host. On a default Windows
install that is the native binaries in `C:\Windows\System32\OpenSSH\`:

- **OpenSSH Client** (`ssh.exe`, `scp.exe`, `ssh-keygen.exe`) ships by default
  on Windows 10 build 1803 and later, and covers the WSL2 driver.
- **`sftp-server.exe`** is part of OpenSSH Server, an [optional Feature on Demand](https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_install_firstuse).
  Only the QEMU driver's reverse-sshfs mounts need it. Install via
  `Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0` from an
  elevated PowerShell, or via Settings → Apps → Optional features.

Installing [Git for Windows](https://gitforwindows.org/)
(`winget install -e --id Git.Git`) remains supported as an alternative
to the native binaries. Use the full Git for Windows installer, not
MinGit — MinGit omits `scp.exe`, `ssh-keygen.exe`, and `cygpath.exe`,
all of which Lima needs.

Lima detects which ssh toolchain is in use on each `limactl start` and
picks both the path form and the matching `sftp-server` binary so both
sides consume the same shape:

- **Cygwin-based ssh** (Git for Windows, MSYS2): paths are converted by
  `cygpath` to a POSIX form like `/c/Users/USER`, respecting any custom
  MSYS2 fstab the user has configured. `sftp-server` is resolved from
  the same toolchain.
- **Native Windows OpenSSH**: paths are returned with forward slashes,
  like `C:/Users/USER` — native `ssh`, `ssh-keygen`, and `scp` accept
  this form directly. `sftp-server.exe` is picked from the sibling
  directory of `ssh.exe`.

When no matching `sftp-server` is found, the hostagent logs a warning at
start and lets `sshocker` auto-detect from `PATH`; a missing
OpenSSH.Server install on a native-only host typically shows up there.

Note: a custom MSYS2 `fstab` that remaps drive prefixes (rare) makes
Cygwin's `/c/...` and native's `C:/...` resolve to different directories
on disk. On such a host, do not swap toolchains between `limactl create`
and `limactl start` on a QEMU instance with a reverse-sshfs mount —
the mount would point at a different directory across the swap. Default
installs without `fstab` overrides are unaffected.
