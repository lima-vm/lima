---
title: VZ
weight: 2
---

| ⚡ Requirement | Lima >= 0.14, macOS >= 13.0 |
|-------------------|-----------------------------|

"vz" option makes use of native virtualization support provided by macOS Virtualization.Framework.

"vz" has been the default driver for macOS hosts since Lima v1.0.

An example configuration (no need to be specified manually):
{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --vm-type=vz
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
vmType: "vz"

base:
- template:_images/ubuntu
- template:_default/mounts
```
{{% /tab %}}
{{< /tabpane >}}

### Memory Ballooning

| ⚡ Requirement | Lima >= 2.1.0, macOS >= 13.0, VZ backend only |
|-------------------|--------------------------------------------|

Memory ballooning dynamically adjusts the guest VM's memory allocation based on actual
usage. When the guest is idle, unused memory is returned to the host. When the guest
needs more memory (detected via PSI — Pressure Stall Information), the balloon grows
automatically.

This is configured under `vmOpts.vz.memoryBalloon`:

```yaml
vmType: "vz"
memory: "8GiB"

vmOpts:
  vz:
    memoryBalloon:
      enabled: true
      min: "2GiB"              # Floor — balloon never shrinks below this
      idleTarget: "3GiB"       # Target when VM is idle
      cooldown: "30s"          # Minimum time between balloon actions
```

When `enabled` is not specified, memory ballooning defaults to disabled. When enabled
with no other fields specified, sensible defaults are derived from the configured
`memory` value (e.g., `min` defaults to 25% of `memory`, `idleTarget` to 33%).

The balloon controller also monitors container CPU/IO activity and swap-in rates to
avoid shrinking memory during active workloads.

### Caveats
- "vz" option is only supported on macOS 13 or above
- Virtualization.framework doesn't support running "intel guest on arm" and vice versa

### Known Issues
- "vz" doesn't support `legacyBIOS: true` option, so guest machine like `centos-stream` and `oraclelinux-8` will not work on Intel Mac.
- When running lima using "vz", `${LIMA_HOME}/<INSTANCE>/serial.log` will not contain kernel boot logs
- On Intel Mac with macOS prior to 13.5, Linux kernel v6.2 (used by Ubuntu 23.04, Fedora 38, etc.) is known to be unbootable on vz.
  kernel v6.3 and later should boot, as long as it is booted via GRUB.
  https://github.com/lima-vm/lima/issues/1577#issuecomment-1565625668
  The issue is fixed in macOS 13.5.
