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
- WSL2 requires a `tar` formatted rootfs archive instead of a VM image

### External tools

Lima uses the native Windows OpenSSH binaries in `C:\Windows\System32\OpenSSH\`:

- **OpenSSH Client** (`ssh.exe`, `scp.exe`, `ssh-keygen.exe`) ships by default
  on Windows 10 build 1803 and later, and covers the WSL2 driver.
- **`sftp-server.exe`** is part of OpenSSH Server, an [optional Feature on Demand](https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_install_firstuse).
  It is only needed for the QEMU driver's reverse-sshfs mounts. Install via
  `Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0` from an
  elevated PowerShell, or via Settings → Apps → Optional features.

No additional Cygwin-style toolchain is required.

Installing [Git for Windows](https://gitforwindows.org/) (`winget install -e --id Git.MinGit`)
remains supported. Lima detects when ssh is a Cygwin-based build and uses
`cygpath` for path translation in that case, which respects any custom MSYS2
fstab the user has configured. On a vanilla Windows install with neither Git
for Windows nor MSYS2, Lima falls back to a native conversion that handles
the common drive-letter case (e.g. `C:\Users\jan` → `/c/Users/jan`).

Note: the ssh toolchain in use is detected at every `limactl start`. Do not
swap between native Windows OpenSSH and Cygwin-based ssh (by adding or
removing Git for Windows / MSYS2 from `PATH`) between `limactl create` and
`limactl start` on a QEMU instance with a reverse-sshfs mount: the
`LocalPath` passed to sftp-server is recomputed on each start and would
change shape across the swap. Sticking with one toolchain for an instance's
lifetime avoids the mismatch.
