---
title: QEMU
weight: 1
---

"qemu" option makes use of QEMU to run guest operating system.

"qemu" is the default driver for Linux hosts.

Recommended QEMU version:
- v8.2.1 or later (macOS)
- v6.2.0 or later (Linux)

On a Windows host, "qemu" needs `ssh`, `scp`, and `ssh-keygen`; the
[OpenSSH client](https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_install_firstuse)
that Windows installs by default provides all three. Mounts must use
reverse-sshfs, because QEMU for Windows has no 9p support. Pinning
[`sftpDriver: openssh-sftp-server`]({{< ref "/docs/config/mount#reverse-sshfs" >}})
also requires `sftp-server.exe`, which comes from the optional OpenSSH Server
feature.

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
