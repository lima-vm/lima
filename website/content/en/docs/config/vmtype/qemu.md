---
title: QEMU
weight: 1
---

"qemu" option makes use of QEMU to run guest operating system.

"qemu" is the default driver for Linux hosts.

Recommended QEMU version:
- v8.2.1 or later (macOS)
- v6.2.0 or later (Linux)

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