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
limactl start --mount-none template:freebsd-15
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

### FreeBSD prior to 16
- No support for mounting host directories.
  Use `limactl cp` or `limactl shell --sync` to share files with the host.
