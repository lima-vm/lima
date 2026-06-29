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
- Windows doesn't ship with ssh.exe, gzip.exe, etc. which are used by Lima at various points. The easiest way around this is to run `winget install -e --id Git.MinGit` (winget is now built in to Windows as well), and add the resulting `C:\Program Files\Git\usr\bin\` directory to your path.

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

2. **Build & Export the Clean Image:**
   When exporting the container rootfs, remove container-specific directories to avoid import issues or unnecessary bloat:
   ```bash
   docker build -t custom-wsl2-image .
   docker create --name custom-wsl2-container custom-wsl2-image
   # Export and filter out .dockerenv and dynamic system directories
   docker export custom-wsl2-container | tar --delete .dockerenv --delete dev --delete proc --delete sys > custom-rootfs.tar
   docker rm custom-wsl2-container
   ```
