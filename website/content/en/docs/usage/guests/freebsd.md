---
title: FreeBSD
weight: 3
---

| ⚡ Requirement | Lima >= 2.1 |
|-------------------|---------|

Running FreeBSD guests is experimentally supported since Lima v2.1.

{{< tabpane text=true >}}
{{% tab header="FreeBSD 15" %}}
```
limactl start template:freebsd-15
```
{{% /tab %}}
{{% tab header="FreeBSD 16 (CURRENT)" %}}
```
limactl start template:experimental/freebsd-16
```
{{% /tab %}}
{{< /tabpane >}}

Prerequisites:
- QEMU
- xorriso (on non-macOS hosts)

## Difference from Linux guests
- Several features are not implemented yet. See [Caveats](#caveats) below.

## Caveats
- No support for automatic port forwarding.  Use `ssh -L` to manually set up port forwarding.
- No support for installing custom `caCerts`
- And more

### FreeBSD prior to 15.1
- No support for mounting host directories.
  Use `limactl cp` or `limactl shell --sync` to share files with the host.

## Plain mode
The guest agent, containerd, and automatic port forwarding are not available on
FreeBSD guests regardless of the mode, so [plain mode](../../config/plain.md)
additionally disables only the host directory mounts (on FreeBSD 15.1 and later).
