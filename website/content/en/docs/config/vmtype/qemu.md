---
title: QEMU
weight: 1
---

"qemu" option makes use of QEMU to run guest operating system.

"qemu" is the default driver for Linux hosts.

Recommended QEMU version:
- v8.2.1 or later (macOS)
- v6.2.0 or later (Linux)

On a Windows host, "qemu" needs an OpenSSH installation for its own `ssh`,
`scp`, and `ssh-keygen`. Mounts there must be reverse-sshfs, because QEMU
for Windows has no 9p support, and reverse-sshfs is the only thing in Lima
that uses `sftp-server`. See
[Windows toolchain]({{< ref "/docs/config/vmtype/wsl2#windows-toolchain" >}})
for which binaries to install and how Lima chooses between them.

An example configuration:
{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --vm-type=qemu
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
vmType: "qemu"

base:
- template:_images/ubuntu
- template:_default/mounts
```
{{% /tab %}}
{{< /tabpane >}}
